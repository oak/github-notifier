package usecase

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// CheckCycleState carries all state that must persist between polling cycles.
// It is owned by the orchestrator and threaded through each Execute call.
type CheckCycleState struct {
	KnownPRs      map[string]bool
	KnownReviews  map[string]map[string]*pullrequest.Review
	ReviewsSeeded bool
	LastCheckTime time.Time
}

// NewCheckCycleState returns a freshly initialised CheckCycleState.
func NewCheckCycleState() CheckCycleState {
	return CheckCycleState{
		KnownPRs:      make(map[string]bool),
		KnownReviews:  make(map[string]map[string]*pullrequest.Review),
		LastCheckTime: time.Now(),
	}
}

// CheckNewPullRequestsUseCase handles fetching and detecting new PRs
// Emits domain events for new PRs instead of directly sending notifications
type CheckNewPullRequestsUseCase struct {
	prRepo         pullrequest.PullRequestRepository
	trackingRepo   pullrequest.PRTrackingRepository
	prFilter       pullrequest.FilterFn
	eventPublisher port.EventPublisher
}

// NewCheckNewPullRequestsUseCase creates a new use case
func NewCheckNewPullRequestsUseCase(
	prRepo pullrequest.PullRequestRepository,
	trackingRepo pullrequest.PRTrackingRepository,
	prFilter pullrequest.FilterFn,
	eventPublisher port.EventPublisher,
) *CheckNewPullRequestsUseCase {
	return &CheckNewPullRequestsUseCase{
		prRepo:         prRepo,
		trackingRepo:   trackingRepo,
		prFilter:       prFilter,
		eventPublisher: eventPublisher,
	}
}

// PRCheckResult contains the results of checking for new PRs
type PRCheckResult struct {
	RequestedReviewPRs []*pullrequest.PullRequest
	UserCreatedPRs     []*pullrequest.PullRequest
}

// Execute fetches PRs and detects new ones.
// It accepts the current inter-cycle state and returns the updated state for the
// next cycle together with the fetched PRs for use by other use cases.
func (uc *CheckNewPullRequestsUseCase) Execute(ctx context.Context, state CheckCycleState) (*PRCheckResult, CheckCycleState, error) {
	uc.seedKnownReviewsFromSnapshots(&state)

	// Fetch PRs from both sources
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching requested review PRs")
		return nil, state, err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching user created PRs")
		return nil, state, err
	}

	// Filter draft PRs if configured
	requestedReviewPRs = uc.prFilter(requestedReviewPRs)
	userCreatedPRs = uc.prFilter(userCreatedPRs)

	// Detect review state changes on all PRs (new and known)
	// This must happen before processNewPRs to detect review changes on known PRs
	uc.detectReviewStateChanges(requestedReviewPRs, &state)
	uc.detectReviewStateChanges(userCreatedPRs, &state)

	// Process requested review PRs
	if err := uc.processNewPRs(requestedReviewPRs, "requested review", &state); err != nil {
		log.Error().Err(err).Msg("Error processing requested review PRs")
	}

	// Process user created PRs
	if err := uc.processNewPRs(userCreatedPRs, "user created", &state); err != nil {
		log.Error().Err(err).Msg("Error processing user created PRs")
	}

	// Advance the timestamp for the next cycle
	state.LastCheckTime = time.Now()

	return &PRCheckResult{
		RequestedReviewPRs: requestedReviewPRs,
		UserCreatedPRs:     userCreatedPRs,
	}, state, nil
}

// seedKnownReviewsFromSnapshots restores review baseline state from persisted
// snapshots one time per process run, so review changes that happened while the
// app was down can still be detected on the first regular cycle.
func (uc *CheckNewPullRequestsUseCase) seedKnownReviewsFromSnapshots(state *CheckCycleState) {
	if state.ReviewsSeeded || uc.trackingRepo == nil {
		return
	}

	prs, err := uc.trackingRepo.LoadAll()
	if err != nil {
		log.Error().Err(err).Msg("Error loading tracked PR snapshots for review baseline seeding")
		return
	}

	for _, pr := range prs {
		// Seed known PR URLs so previously tracked PRs are not re-detected as new
		// after process restart.
		state.KnownPRs[pr.URL()] = true

		if _, exists := state.KnownReviews[pr.URL()]; exists {
			continue
		}

		reviews := make(map[string]*pullrequest.Review, len(pr.Reviews()))
		for login, review := range pr.Reviews() {
			reviewer, authorErr := pullrequest.NewAuthor(login)
			if authorErr != nil {
				log.Error().Err(authorErr).Msgf("Skipping invalid reviewer %q while seeding review baseline", login)
				continue
			}

			reviews[login] = pullrequest.NewReview(reviewer, review.State(), review.SubmittedAt())
		}

		state.KnownReviews[pr.URL()] = reviews
	}

	state.ReviewsSeeded = true
}

// processNewPRs finds new PRs and emits appropriate events
func (uc *CheckNewPullRequestsUseCase) processNewPRs(prs []*pullrequest.PullRequest, category string, state *CheckCycleState) error {
	var newPRs []*pullrequest.PullRequest
	for _, pr := range prs {
		if !state.KnownPRs[pr.URL()] {
			newPRs = append(newPRs, pr)
		}
		// Always track as known so MarkPullRequestAsUnseen can't cause re-detection
		state.KnownPRs[pr.URL()] = true
	}

	if len(newPRs) == 0 {
		return nil
	}

	log.Info().Msgf("Found %d new %s PRs", len(newPRs), category)

	// Classify PRs: truly new vs. PRs with new activity
	trulyNewPRs, prsWithActivity := pullrequest.ClassifyPRs(newPRs, state.LastCheckTime)

	// Mark truly new PRs as newly detected (raises domain events)
	for _, pr := range trulyNewPRs {
		for _, event := range pr.MarkAsNewlyDetected() {
			if err := uc.eventPublisher.Publish(event); err != nil {
				log.Error().Err(err).Msg("Error publishing new PR event")
			}
		}
	}

	// Mark all new PRs as seen in the tracking repo
	// This sets the initial seen state for UI display
	for _, pr := range newPRs {
		pr.MarkAsSeen()
	}

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
func (uc *CheckNewPullRequestsUseCase) detectReviewStateChanges(prs []*pullrequest.PullRequest, state *CheckCycleState) {
	for _, pr := range prs {
		currentReviews := pr.Reviews()
		knownReviews, exists := state.KnownReviews[pr.URL()]

		if !exists {
			// First time seeing this PR — save baseline, no events
			state.KnownReviews[pr.URL()] = currentReviews
			continue
		}

		// Restore known reviews on the fresh PR object (without events)
		pr.SetInitialReviews(knownReviews)

		// Now apply each current review via AddReview (returns event on state change)
		var reviewEvents []pullrequest.Event
		for _, review := range currentReviews {
			reviewEvents = append(reviewEvents, pr.AddReview(review)...)
		}

		// Publish events returned by AddReview.
		for _, event := range reviewEvents {
			if err := uc.eventPublisher.Publish(event); err != nil {
				log.Error().Err(err).Msg("Error publishing review state changed event")
			}
		}

		// Update known reviews to current state
		state.KnownReviews[pr.URL()] = pr.Reviews()
	}
}
