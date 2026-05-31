package usecase

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
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

// TrackPRs records the current open PR set for next-cycle comparison.
//
// It performs a merge: identity and status fields are taken from the supplied
// PR objects while enrichment fields (HeadCommitSHA, PipelineStatus,
// LastActivityCheck) are preserved from the previously-saved snapshots. This
// ensures that TrackPullRequestActivityUseCase always sees the PREVIOUS
// cycle's enrichment state when it loads the repository — which is what it
// needs to detect cross-cycle changes correctly.
func (uc *DetectClosedPullRequestsUseCase) TrackPRs(prs []*pullrequest.PullRequest) {
	existing, err := uc.trackingRepo.LoadAll()
	if err != nil {
		log.Error().Err(err).Msg("DetectClosedPRs: failed to load existing snapshots for merge; saving without enrichment data")
		existing = nil
	}

	// Build lookup map for previous enrichment data
	prevByURL := make(map[string]pullrequest.PullRequest, len(existing))
	for _, s := range existing {
		prevByURL[s.URL()] = s
	}

	snapshots := make([]pullrequest.PullRequest, 0, len(prs))
	for _, pr := range prs {
		// Preserve enrichment fields from the previous snapshot so that
		// TrackPullRequestActivityUseCase can correctly detect changes.
		if prev, ok := prevByURL[pr.URL()]; ok {
			pr.HeadCommitSHA = prev.HeadCommitSHA
			pr.PipelineStatus = prev.PipelineStatus
			pr.LastActivityCheck = prev.LastActivityCheck
		}
		snapshots = append(snapshots, pr)
	}

	if err := uc.trackingRepo.Save(snapshots); err != nil {
		log.Error().Err(err).Msg("DetectClosedPRs: failed to save tracked PR snapshots")
	}
}

// Execute compares the current open PR list against the previously-saved
// snapshot set to detect merged/closed PRs.
func (uc *DetectClosedPullRequestsUseCase) Execute(ctx context.Context, currentPRs []*pullrequest.PullRequest) error {
	prs, err := uc.trackingRepo.LoadAll()
	if err != nil {
		return err
	}

	if len(prs) == 0 {
		return nil // Nothing tracked yet — first cycle
	}

	// Build a set of currently open PR URLs
	currentURLs := make(map[string]bool, len(currentPRs))
	for _, pr := range currentPRs {
		currentURLs[pr.URL()] = true
	}

	// Find prs for PRs that are no longer in the open list
	var missing []pullrequest.PullRequest
	for _, pr := range prs {
		if !currentURLs[pr.URL()] {
			missing = append(missing, pr)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	log.Info().Msgf("Detected %d PR(s) missing from open list, checking final status", len(missing))

	// Track which missing PRs were confirmed closed/merged so we can remove
	// them from the repository immediately (avoids re-processing on the next
	// Execute call in the same cycle).
	processedURLs := make(map[string]bool, len(missing))

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
			processedURLs[pr.URL()] = true

		case pullrequest.StatusClosed:
			log.Info().Msgf("PR %s was closed", pr.URL())
			for _, event := range pr.Close() {
				if err := uc.eventPublisher.Publish(event); err != nil {
					log.Error().Err(err).Msgf("Error publishing event for PR %s", pr.URL())
				}
			}
			processedURLs[pr.URL()] = true

		case pullrequest.StatusOpen:
			// PR is still open but not in our fetch results — transient
			// (pagination lag, search index lag). Keep tracking.
			log.Debug().Msgf("PR %s still open but not in fetch results, keeping tracked", pr.URL())
		}
	}

	if len(processedURLs) == 0 {
		return nil
	}

	// Remove confirmed closed/merged PRs from the repository immediately so
	// that a subsequent Execute call does not re-process them.
	remaining := make([]pullrequest.PullRequest, 0, len(prs)-len(processedURLs))
	for _, pr := range prs {
		if !processedURLs[pr.URL()] {
			remaining = append(remaining, pr)
		}
	}
	if saveErr := uc.trackingRepo.Save(remaining); saveErr != nil {
		log.Error().Err(saveErr).Msg("DetectClosedPRs: failed to remove closed PRs from tracking repo")
	}

	return nil
}
