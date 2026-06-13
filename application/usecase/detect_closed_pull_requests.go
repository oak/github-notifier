package usecase

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/oak/github-notifier/application/port"
	"github.com/oak/github-notifier/domain/pullrequest"
)

// DetectClosedPullRequestsUseCase detects PRs that have been merged or closed
// by comparing the current fetch results against the set of open PRs persisted
// at the end of the previous check cycle.
//
// When a tracked PR disappears from the open PR list it queries GitHub for the
// final status and emits the appropriate domain events.
//
// Tracking cleanup (e.g. removing from seen state) is handled by event handlers
// subscribed to Merged and Closed events.
type DetectClosedPullRequestsUseCase struct {
	prRepo         pullrequest.PullRequestRepository
	trackingRepo   pullrequest.PRTrackingRepository
	eventPublisher port.EventPublisher
}

// NewDetectClosedPullRequestsUseCase creates a new use case.
func NewDetectClosedPullRequestsUseCase(
	prRepo pullrequest.PullRequestRepository,
	trackingRepo pullrequest.PRTrackingRepository,
	eventPublisher port.EventPublisher,
) *DetectClosedPullRequestsUseCase {
	return &DetectClosedPullRequestsUseCase{
		prRepo:         prRepo,
		trackingRepo:   trackingRepo,
		eventPublisher: eventPublisher,
	}
}

// Execute compares the current open PR list against the previously-saved
// snapshot set to detect merged/closed PRs. Returns a list of closed/merged
// PR URLs so the caller can clean up cycle state (KnownPRs, KnownReviews).
//
// After detection, currentPRs is always persisted as the new snapshot for the
// next cycle — no separate tracking call is required from the caller.
func (uc *DetectClosedPullRequestsUseCase) Execute(ctx context.Context, currentPRs []*pullrequest.PullRequest) ([]string, error) {
	prev, err := uc.trackingRepo.LoadAll()
	if err != nil {
		return nil, err
	}

	var closedMergedURLs []string

	if len(prev) > 0 {
		// Build a set of currently open PR URLs
		currentURLs := make(map[string]bool, len(currentPRs))
		for _, pr := range currentPRs {
			currentURLs[pr.URL()] = true
		}

		// Find PRs that are no longer in the open list
		var missing []pullrequest.PullRequest
		for _, pr := range prev {
			if !currentURLs[pr.URL()] {
				missing = append(missing, *pr)
			}
		}

		if len(missing) > 0 {
			log.Info().Msgf("Detected %d PR(s) missing from open list, checking final status", len(missing))

			for _, pr := range missing {
				repo := pr.Repository()
				status, fetchErr := uc.prRepo.FetchPRStatus(repo.Owner(), repo.Name(), pr.Number())
				if fetchErr != nil {
					// Don't remove from tracking on API errors — avoid false positives.
					log.Error().Err(fetchErr).Msgf("Error fetching status for PR %s, skipping", pr.URL())
					continue
				}

				switch status {
				case pullrequest.StatusMerged:
					log.Info().Msgf("PR %s was merged", pr.URL())
					for _, event := range pr.Merge() {
						if err := uc.eventPublisher.Publish(event); err != nil {
							log.Error().Err(err).Msgf("Error publishing event for PR %s", pr.URL())
						}
					}
					closedMergedURLs = append(closedMergedURLs, pr.URL())

				case pullrequest.StatusClosed:
					log.Info().Msgf("PR %s was closed", pr.URL())
					for _, event := range pr.Close() {
						if err := uc.eventPublisher.Publish(event); err != nil {
							log.Error().Err(err).Msgf("Error publishing event for PR %s", pr.URL())
						}
					}
					closedMergedURLs = append(closedMergedURLs, pr.URL())

				case pullrequest.StatusOpen:
					// PR is still open but not in our fetch results — transient
					// (pagination lag, search index lag). Keep tracking.
					log.Debug().Msgf("PR %s still open but not in fetch results, keeping tracked", pr.URL())
				}
			}
		}
	}

	// Always persist currentPRs as the new snapshot for the next cycle.
	// Enrichment fields (HeadCommitSHA, PipelineStatus, LastActivityCheck) are
	// preserved from prev so TrackPullRequestActivityUseCase can detect
	// cross-cycle changes correctly. Snapshot copies are value copies so the
	// caller's live PR objects are never mutated.
	uc.saveSnapshot(currentPRs, prev)

	return closedMergedURLs, nil
}

// saveSnapshot persists currentPRs as the tracking baseline for the next cycle,
// preserving enrichment fields from the previous snapshot where available.
func (uc *DetectClosedPullRequestsUseCase) saveSnapshot(currentPRs, prev []*pullrequest.PullRequest) {
	prevByURL := make(map[string]*pullrequest.PullRequest, len(prev))
	for _, s := range prev {
		prevByURL[s.URL()] = s
	}

	snapshots := make([]*pullrequest.PullRequest, 0, len(currentPRs))
	for _, pr := range currentPRs {
		snap := *pr
		if p, ok := prevByURL[pr.URL()]; ok {
			snap.SetHeadCommitSHA(p.HeadCommitSHA())
			snap.SetPipelineStatus(p.PipelineStatus())
			snap.SetLastActivityCheck(p.LastActivityCheck())
		}
		snapshots = append(snapshots, &snap)
	}

	if err := uc.trackingRepo.Save(snapshots); err != nil {
		log.Error().Err(err).Msg("DetectClosedPRs: failed to save tracked PR snapshots")
	}
}
