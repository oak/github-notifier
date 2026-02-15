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
	prRepo            pullrequest.PullRequestRepository
	scheduler         *pullrequest.ActivityCheckScheduler
	trackingService   *pullrequest.TrackingService
	eventPublisher    port.EventPublisher
	knownHeadSHAs     map[string]string // PR URL → last known head commit SHA
	authenticatedUser string            // GitHub login — used to filter self-activity for unseen marking
}

// NewTrackPullRequestActivityUseCase creates a new use case.
// authenticatedUser is the GitHub login of the current user; self-authored activities
// are not considered "new activity" for the purposes of marking PRs as unseen.
func NewTrackPullRequestActivityUseCase(
	prRepo pullrequest.PullRequestRepository,
	scheduler *pullrequest.ActivityCheckScheduler,
	trackingService *pullrequest.TrackingService,
	eventPublisher port.EventPublisher,
	authenticatedUser string,
) *TrackPullRequestActivityUseCase {
	return &TrackPullRequestActivityUseCase{
		prRepo:            prRepo,
		scheduler:         scheduler,
		trackingService:   trackingService,
		eventPublisher:    eventPublisher,
		knownHeadSHAs:     make(map[string]string),
		authenticatedUser: authenticatedUser,
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

	log.Info().Msgf("Activity scheduler: checking %d/%d PRs (%d recent, %d stale due for check, %d skipped)",
		len(prsToCheck), len(prs), scheduleResult.RecentCount, scheduleResult.StaleCount, scheduleResult.SkippedCount)

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

	// Collect and publish all domain events from enrichment.
	// ActivityDetected events are raised by the aggregate when activities are added.
	// The notification handler filters out self-authored activities.
	for _, pr := range prsToCheck {
		for _, event := range pr.CollectEvents() {
			if err := uc.eventPublisher.Publish(event); err != nil {
				log.Error().Err(err).Msg("Error publishing activity event")
			}
		}
	}

	// Find PRs with new activity by others (filter out self-authored activities)
	// to decide which PRs should be marked unseen (asterisks in UI)
	var prsWithNewActivity []*pullrequest.PullRequest
	for _, pr := range prsToCheck {
		if uc.hasActivityByOthers(pr, lastCheckTime) {
			prsWithNewActivity = append(prsWithNewActivity, pr)
		}
	}

	if len(prsWithNewActivity) == 0 {
		log.Info().Msgf("No new activity detected on checked PRs")
		return nil
	}

	log.Info().Msgf("Found %d PRs with new activity", len(prsWithNewActivity))

	// Mark PRs with new activity as unseen (to show asterisks)
	for _, pr := range prsWithNewActivity {
		if err := uc.trackingService.MarkPullRequestAsUnseen(pr); err != nil {
			log.Error().Err(err).Msg("Error marking PR as unseen")
		}
	}

	return nil
}

// hasActivityByOthers returns true if the PR has any activities since lastCheckTime
// that were NOT authored by the authenticated user. Self-authored activities are
// domain facts but should not trigger unseen marking or notifications.
func (uc *TrackPullRequestActivityUseCase) hasActivityByOthers(pr *pullrequest.PullRequest, since time.Time) bool {
	for _, activity := range pr.ActivitiesSince(since) {
		if uc.authenticatedUser == "" || activity.Author().Login() != uc.authenticatedUser {
			return true
		}
	}
	return false
}
