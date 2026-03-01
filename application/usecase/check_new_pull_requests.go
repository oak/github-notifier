package usecase

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// CheckNewPullRequestsUseCase handles fetching and detecting new PRs
// Emits domain events for new PRs instead of directly sending notifications
type CheckNewPullRequestsUseCase struct {
	prRepo          pullrequest.PullRequestRepository
	trackingService *pullrequest.TrackingService
	prFilter        pullrequest.FilterFn
	eventPublisher  port.EventPublisher
	lastCheckTime   time.Time
	knownPRs        map[string]bool                           // Tracks all PRs ever encountered by URL, independent of seen repository
	knownReviews    map[string]map[string]*pullrequest.Review // PR URL -> reviewer login -> review state
}

// NewCheckNewPullRequestsUseCase creates a new use case
func NewCheckNewPullRequestsUseCase(
	prRepo pullrequest.PullRequestRepository,
	trackingService *pullrequest.TrackingService,
	prFilter pullrequest.FilterFn,
	eventPublisher port.EventPublisher,
) *CheckNewPullRequestsUseCase {
	return &CheckNewPullRequestsUseCase{
		prRepo:          prRepo,
		trackingService: trackingService,
		prFilter:        prFilter,
		eventPublisher:  eventPublisher,
		lastCheckTime:   time.Now(),
		knownPRs:        make(map[string]bool),
		knownReviews:    make(map[string]map[string]*pullrequest.Review),
	}
}

// PRCheckResult contains the results of checking for new PRs
type PRCheckResult struct {
	RequestedReviewPRs []*pullrequest.PullRequest
	UserCreatedPRs     []*pullrequest.PullRequest
}

// Execute fetches PRs and detects new ones
// Returns the fetched PRs for use by other use cases (activity tracking, display)
func (uc *CheckNewPullRequestsUseCase) Execute(ctx context.Context) (*PRCheckResult, error) {
	// Fetch PRs from both sources
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching requested review PRs")
		return nil, err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching user created PRs")
		return nil, err
	}

	// Filter draft PRs if configured
	requestedReviewPRs = uc.prFilter(requestedReviewPRs)
	userCreatedPRs = uc.prFilter(userCreatedPRs)

	// Detect review state changes on all PRs (new and known)
	// This must happen before processNewPRs to detect review changes on known PRs
	uc.detectReviewStateChanges(requestedReviewPRs)
	uc.detectReviewStateChanges(userCreatedPRs)

	// Process requested review PRs
	if err := uc.processNewPRs(requestedReviewPRs, "requested review"); err != nil {
		log.Error().Err(err).Msg("Error processing requested review PRs")
	}

	// Process user created PRs
	if err := uc.processNewPRs(userCreatedPRs, "user created"); err != nil {
		log.Error().Err(err).Msg("Error processing user created PRs")
	}

	// Update last check time
	uc.lastCheckTime = time.Now()

	return &PRCheckResult{
		RequestedReviewPRs: requestedReviewPRs,
		UserCreatedPRs:     userCreatedPRs,
	}, nil
}

// processNewPRs finds new PRs and emits appropriate events
func (uc *CheckNewPullRequestsUseCase) processNewPRs(prs []*pullrequest.PullRequest, category string) error {
	// Find PRs that are genuinely new (never encountered before).
	// We check both our own knownPRs set AND the seen repository.
	// The knownPRs set prevents false re-detections that occur when the
	// activity tracking use case calls MarkPullRequestAsUnseen (which removes
	// PRs from the seen repository so the UI can show asterisks for unread
	// activity). The seen repository check handles app restarts where
	// knownPRs is empty but the seen repo has persisted data.
	var newPRs []*pullrequest.PullRequest
	for _, pr := range prs {
		if !uc.knownPRs[pr.URL()] && !uc.trackingService.HasBeenSeen(pr.Identifier()) {
			newPRs = append(newPRs, pr)
		}
		// Always track as known so MarkPullRequestAsUnseen can't cause re-detection
		uc.knownPRs[pr.URL()] = true
	}

	if len(newPRs) == 0 {
		return nil
	}

	log.Info().Msgf("Found %d new %s PRs", len(newPRs), category)

	// Classify PRs: truly new vs. PRs with new activity
	trulyNewPRs, prsWithActivity := pullrequest.ClassifyPRs(newPRs, uc.lastCheckTime)

	// Mark truly new PRs as newly detected (raises domain events)
	for _, pr := range trulyNewPRs {
		pr.MarkAsNewlyDetected()

		// Drain and publish events from the aggregate
		for _, event := range pr.DrainEvents() {
			if err := uc.eventPublisher.Publish(event); err != nil {
				log.Error().Err(err).Msg("Error publishing new PR event")
			}
		}
	}

	// Mark all new PRs as seen in the tracking service
	// This sets the initial seen state for UI display
	uc.trackingService.MarkPullRequestsAsSeen(newPRs)

	// PRs with activity will trigger activity events via TrackPullRequestActivityUseCase
	if len(prsWithActivity) > 0 {
		log.Info().Msgf("%d PRs have new activity (handled separately)", len(prsWithActivity))
	}

	return nil
}

// detectReviewStateChanges compares current reviews with known reviews for each PR.
// For known PRs, it restores the previous review state, applies new reviews via AddReview
// (which raises ReviewStateChanged events when the state actually changes), then collects
// and publishes any resulting events.
// For new PRs (first time seen), it just stores the reviews as the baseline.
func (uc *CheckNewPullRequestsUseCase) detectReviewStateChanges(prs []*pullrequest.PullRequest) {
	for _, pr := range prs {
		currentReviews := pr.Reviews()
		knownReviews, exists := uc.knownReviews[pr.URL()]

		if !exists {
			// First time seeing this PR — save baseline, no events
			uc.knownReviews[pr.URL()] = currentReviews
			continue
		}

		// Restore known reviews on the fresh PR object (without events)
		pr.SetInitialReviews(knownReviews)

		// Now apply each current review via AddReview (raises events on state change)
		for _, review := range currentReviews {
			pr.AddReview(review)
		}

		// Drain and publish events raised by AddReview.
		for _, event := range pr.DrainEvents() {
			if err := uc.eventPublisher.Publish(event); err != nil {
				log.Error().Err(err).Msg("Error publishing review state changed event")
			}
		}

		// Update known reviews to current state
		uc.knownReviews[pr.URL()] = pr.Reviews()
	}
}
