package usecase

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// TrackPullRequestActivityUseCase handles checking for new activity on PRs
// Uses a two-tier scheduling strategy to optimize API calls
type TrackPullRequestActivityUseCase struct {
	prRepo          pullrequest.PullRequestRepository
	scheduler       *pullrequest.ActivityCheckScheduler
	trackingService *pullrequest.TrackingService
	eventPublisher  port.EventPublisher
	knownHeadSHAs   map[string]string // PR URL → last known head commit SHA
}

// NewTrackPullRequestActivityUseCase creates a new use case
func NewTrackPullRequestActivityUseCase(
	prRepo pullrequest.PullRequestRepository,
	scheduler *pullrequest.ActivityCheckScheduler,
	trackingService *pullrequest.TrackingService,
	eventPublisher port.EventPublisher,
) *TrackPullRequestActivityUseCase {
	return &TrackPullRequestActivityUseCase{
		prRepo:          prRepo,
		scheduler:       scheduler,
		trackingService: trackingService,
		eventPublisher:  eventPublisher,
		knownHeadSHAs:   make(map[string]string),
	}
}

// Execute checks for new activity on PRs using two-tier scheduling
// Only checks PRs that are due based on the scheduling strategy
func (uc *TrackPullRequestActivityUseCase) Execute(
	ctx context.Context,
	prs []*pullrequest.PullRequest,
	lastCheckTime time.Time,
) error {
	if len(prs) == 0 {
		return nil
	}

	// Determine which PRs to check based on two-tier scheduling
	scheduleResult := uc.scheduler.DeterminePRsToCheck(prs)
	prsToCheck := scheduleResult.PRsToCheck

	if len(prsToCheck) == 0 {
		log.Info().Msgf("Activity tracking: No PRs due for checking")
		return nil
	}

	// Restore known head commit SHAs on the fresh PR objects
	// (PR objects are recreated each cycle, so we need to carry state forward)
	for _, pr := range prsToCheck {
		if sha, ok := uc.knownHeadSHAs[pr.URL()]; ok {
			pr.SetInitialHeadCommitSHA(sha)
		}
	}

	// Enrich PRs with activities since last check
	// This also updates the head commit SHA and creates push activities if head changed
	if err := uc.prRepo.EnrichWithActivities(prsToCheck, lastCheckTime); err != nil {
		log.Error().Err(err).Msg("Error enriching PRs with activities")
		return err
	}

	// Save updated head commit SHAs for next cycle
	for _, pr := range prsToCheck {
		if sha := pr.HeadCommitSHA(); sha != "" {
			uc.knownHeadSHAs[pr.URL()] = sha
		}
	}

	// Mark these PRs as checked in the scheduler
	uc.scheduler.MarkChecked(prsToCheck)

	// Find PRs with new activity and emit events
	var prsWithNewActivity []*pullrequest.PullRequest
	for _, pr := range prsToCheck {
		if pr.HasActivitiesSince(lastCheckTime) {
			prsWithNewActivity = append(prsWithNewActivity, pr)
		}
	}

	if len(prsWithNewActivity) == 0 {
		log.Info().Msgf("No new activity detected on checked PRs")
		return nil
	}

	log.Info().Msgf("Found %d PRs with new activity", len(prsWithNewActivity))

	// Mark PRs with new activity as unseen (to show asterisks and trigger notifications)
	for _, pr := range prsWithNewActivity {
		if err := uc.trackingService.MarkPullRequestAsUnseen(pr); err != nil {
			log.Error().Err(err).Msg("Error marking PR as unseen")
		}

		// Record activity (raises domain event)
		pr.RecordNewActivity()

		// Collect and publish events from the aggregate
		for _, event := range pr.CollectEvents() {
			if err := uc.eventPublisher.Publish(event); err != nil {
				log.Error().Err(err).Msg("Error publishing activity event")
			}
		}
	}

	return nil
}
