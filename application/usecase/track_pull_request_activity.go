package usecase

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// TrackPullRequestActivityUseCase handles checking for new activity on PRs.
// Uses a two-tier scheduling strategy to optimize API calls.
//
// Enrichment state (head-commit SHA, pipeline status, last-activity check
// timestamp) is persisted via PRTrackingRepository so that it survives process
// restarts and is also shared with DetectClosedPullRequestsUseCase, which
// merges this data into the snapshots it saves each cycle.
type TrackPullRequestActivityUseCase struct {
	prRepo            pullrequest.PullRequestRepository
	trackingRepo      pullrequest.PRTrackingRepository
	scheduler         *pullrequest.ActivityCheckScheduler
	trackingService   *pullrequest.TrackingService
	eventPublisher    port.EventPublisher
	authenticatedUser string // GitHub login — used to filter self-activity for unseen marking
}

// NewTrackPullRequestActivityUseCase creates a new use case.
// authenticatedUser is the GitHub login of the current user; self-authored activities
// are not considered "new activity" for the purposes of marking PRs as unseen.
func NewTrackPullRequestActivityUseCase(
	prRepo pullrequest.PullRequestRepository,
	trackingRepo pullrequest.PRTrackingRepository,
	scheduler *pullrequest.ActivityCheckScheduler,
	trackingService *pullrequest.TrackingService,
	eventPublisher port.EventPublisher,
	authenticatedUser string,
) *TrackPullRequestActivityUseCase {
	return &TrackPullRequestActivityUseCase{
		prRepo:            prRepo,
		trackingRepo:      trackingRepo,
		scheduler:         scheduler,
		trackingService:   trackingService,
		eventPublisher:    eventPublisher,
		authenticatedUser: authenticatedUser,
	}
}

// Execute checks for new activity on PRs using two-tier scheduling.
// Only checks PRs that are due based on the scheduling strategy.
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

	// Load persisted enrichment state for all tracked PRs.
	// DetectClosedPRsUseCase.TrackPRs writes this at the end of each cycle.
	snapshots, loadErr := uc.trackingRepo.LoadAll()
	if loadErr != nil {
		log.Error().Err(loadErr).Msg("Activity tracker: failed to load snapshots; proceeding without prior state")
	}
	snapByURL := make(map[string]pullrequest.PRStateSnapshot, len(snapshots))
	for _, s := range snapshots {
		snapByURL[s.URL] = s
	}

	// Restore known enrichment state on fresh PR objects.
	// (PR objects are recreated each cycle by the GitHub adapter, so we carry
	// state forward from the persisted snapshots.)
	//
	// For pipeline status: the initial search query seeds the PR with the
	// current GitHub state via SetInitialPipelineStatus. We OVERRIDE that with
	// the previous cycle's stored value so that UpdatePipelineStatus (called
	// inside EnrichWithActivities) can correctly detect a change vs. a no-op.
	// If we have no stored value yet, reset to Unknown so the first real status
	// from the timeline query fires the initial transition event.
	for _, pr := range prsToCheck {
		if snap, ok := snapByURL[pr.URL()]; ok {
			pr.SetInitialHeadCommitSHA(snap.HeadCommitSHA)
			pr.SetInitialPipelineStatus(snap.PipelineStatus)
			uc.scheduler.SeedLastChecked(pr.URL(), snap.LastActivityCheck)
		} else {
			// No stored state — reset pipeline to Unknown so the first status
			// from the batched timeline query fires a transition event.
			pr.SetInitialPipelineStatus(pullrequest.PipelineStatusUnknown)
		}
	}

	// Enrich PRs with activities since last check.
	// This also updates the head-commit SHA and creates push activities if the
	// head changed. Returns all domain events raised during enrichment.
	enrichEvents, err := uc.prRepo.EnrichWithActivities(prsToCheck, lastCheckTime)
	if err != nil {
		log.Error().Err(err).Msg("Error enriching PRs with activities")
		return err
	}

	// Mark these PRs as checked in the scheduler
	uc.scheduler.MarkChecked(prsToCheck)

	// Persist updated enrichment state so the next cycle (and DetectClosedPRs)
	// can see the freshly-enriched values.
	//
	// We update only the PRs we just checked; everything else keeps its
	// snapshot unchanged.
	checkedByURL := make(map[string]*pullrequest.PullRequest, len(prsToCheck))
	for _, pr := range prsToCheck {
		checkedByURL[pr.URL()] = pr
	}
	updatedSnapshots := make([]pullrequest.PRStateSnapshot, 0, len(snapshots))
	for _, s := range snapshots {
		if pr, ok := checkedByURL[s.URL]; ok {
			updated := pr.ToSnapshot()
			// Preserve identity/display fields from the original snapshot to
			// avoid clobbering data that TrackPRs saved (e.g. IsDraft changes).
			updatedSnapshots = append(updatedSnapshots, updated)
			delete(checkedByURL, s.URL) // mark as handled
		} else {
			updatedSnapshots = append(updatedSnapshots, s)
		}
	}
	// Any PRs that were checked but had no prior snapshot (new this cycle):
	for _, pr := range checkedByURL {
		updatedSnapshots = append(updatedSnapshots, pr.ToSnapshot())
	}
	if saveErr := uc.trackingRepo.Save(updatedSnapshots); saveErr != nil {
		log.Error().Err(saveErr).Msg("Activity tracker: failed to save updated snapshots")
	}

	// Publish all domain events returned from enrichment.
	// ActivityDetected, PipelineStatusChanged etc. are returned directly by the aggregate commands.
	for _, event := range enrichEvents {
		if err := uc.eventPublisher.Publish(event); err != nil {
			log.Error().Err(err).Msg("Error publishing activity event")
		}
	}

	// Find PRs with new activity by others (filter out self-authored activities)
	// to decide which PRs should be marked unseen (asterisks in UI).
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
