package usecase

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// DetectClosedPullRequestsUseCase detects PRs that have been merged or closed
// by comparing locally tracked PRs against the current fetch results.
// When a tracked PR disappears from the open PR list, it queries GitHub for
// the final status and emits the appropriate domain events.
// Tracking cleanup (e.g. removing from seen state) is handled by event handlers
// subscribed to Merged and Closed events.
type DetectClosedPullRequestsUseCase struct {
	prRepo         pullrequest.PullRequestRepository
	eventPublisher port.EventPublisher
	trackedPRs     map[string]*pullrequest.PullRequest // URL -> PR (all PRs we've ever tracked)
}

// NewDetectClosedPullRequestsUseCase creates a new use case
func NewDetectClosedPullRequestsUseCase(
	prRepo pullrequest.PullRequestRepository,
	eventPublisher port.EventPublisher,
) *DetectClosedPullRequestsUseCase {
	return &DetectClosedPullRequestsUseCase{
		prRepo:         prRepo,
		eventPublisher: eventPublisher,
		trackedPRs:     make(map[string]*pullrequest.PullRequest),
	}
}

// TrackPRs records PRs that are currently being tracked.
// Called after each fetch to build up the set of known PRs.
func (uc *DetectClosedPullRequestsUseCase) TrackPRs(prs []*pullrequest.PullRequest) {
	for _, pr := range prs {
		uc.trackedPRs[pr.URL()] = pr
	}
}

// Execute compares currently fetched open PRs against tracked PRs to detect
// merged/closed PRs. For each missing PR, it queries GitHub for the final
// status and emits the appropriate event.
func (uc *DetectClosedPullRequestsUseCase) Execute(ctx context.Context, currentPRs []*pullrequest.PullRequest) error {
	if len(uc.trackedPRs) == 0 {
		return nil // Nothing tracked yet
	}

	// Build a set of currently open PR URLs
	currentURLs := make(map[string]bool, len(currentPRs))
	for _, pr := range currentPRs {
		currentURLs[pr.URL()] = true
	}

	// Find tracked PRs that are no longer in the open list
	var missingPRs []*pullrequest.PullRequest
	for url, pr := range uc.trackedPRs {
		if !currentURLs[url] {
			missingPRs = append(missingPRs, pr)
		}
	}

	if len(missingPRs) == 0 {
		return nil
	}

	log.Info().Msgf("Detected %d PR(s) missing from open list, checking final status", len(missingPRs))

	// Query GitHub for the final status of each missing PR
	for _, pr := range missingPRs {
		repo := pr.Repository()
		status, err := uc.prRepo.FetchPRStatus(repo.Owner(), repo.Name(), pr.Number())
		if err != nil {
			// Don't remove from tracked on API errors — avoid false positives
			log.Error().Err(err).Msgf("Error fetching status for PR %s, skipping", pr.URL())
			continue
		}

		switch status {
		case pullrequest.StatusMerged:
			log.Info().Msgf("PR %s was merged", pr.URL())
			pr.Merge()
			uc.publishEvents(pr)
			uc.cleanup(pr)

		case pullrequest.StatusClosed:
			log.Info().Msgf("PR %s was closed", pr.URL())
			pr.Close()
			uc.publishEvents(pr)
			uc.cleanup(pr)

		case pullrequest.StatusOpen:
			// PR is still open but not in our fetch results — this can happen
			// transiently (e.g., pagination, search index lag). Keep tracking.
			log.Debug().Msgf("PR %s still open but not in fetch results, keeping tracked", pr.URL())
		}
	}

	return nil
}

// publishEvents collects and publishes all pending domain events from the PR aggregate.
// Merge() and Close() raise their respective Merged/Closed events on the aggregate;
// this method forwards them to the event bus so all handlers receive them.
func (uc *DetectClosedPullRequestsUseCase) publishEvents(pr *pullrequest.PullRequest) {
	for _, event := range pr.CollectEvents() {
		if err := uc.eventPublisher.Publish(event); err != nil {
			log.Error().Err(err).Msgf("Error publishing event for PR %s", pr.URL())
		}
	}
}

// cleanup removes a PR from the local tracked set.
// Tracking state cleanup (seen repository) is handled by the TrackingEventHandler
// in response to the Merged/Closed events emitted above.
func (uc *DetectClosedPullRequestsUseCase) cleanup(pr *pullrequest.PullRequest) {
	delete(uc.trackedPRs, pr.URL())
}
