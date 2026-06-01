package usecase

import (
	"context"
	"sync"
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
	eventPublisher    port.EventPublisher
	authenticatedUser string // GitHub login — used to filter self-activity for unseen marking
	mu                sync.RWMutex
	ignoreConfig      *pullrequest.IgnoreConfig // may be nil; safe to update concurrently
}

// NewTrackPullRequestActivityUseCase creates a new use case.
// authenticatedUser is the GitHub login of the current user; self-authored activities
// are not considered "new activity" for the purposes of marking PRs as unseen.
func NewTrackPullRequestActivityUseCase(
	prRepo pullrequest.PullRequestRepository,
	trackingRepo pullrequest.PRTrackingRepository,
	scheduler *pullrequest.ActivityCheckScheduler,
	eventPublisher port.EventPublisher,
	authenticatedUser string,
) *TrackPullRequestActivityUseCase {
	return &TrackPullRequestActivityUseCase{
		prRepo:            prRepo,
		trackingRepo:      trackingRepo,
		scheduler:         scheduler,
		eventPublisher:    eventPublisher,
		authenticatedUser: authenticatedUser,
	}
}

// UpdateIgnoreConfig atomically replaces the active ignore config used to decide
// whether a PR should be marked as unseen. Safe to call from any goroutine.
func (uc *TrackPullRequestActivityUseCase) UpdateIgnoreConfig(cfg *pullrequest.IgnoreConfig) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.ignoreConfig = cfg
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
	allCurrentPRs := prs
	scheduleResult := uc.scheduler.DeterminePRsToCheck(allCurrentPRs)
	prsToCheck := scheduleResult.PRsToCheck

	log.Info().Msgf("Activity scheduler: checking %d/%d PRs (%d recent, %d stale due for check, %d skipped)",
		len(prsToCheck), len(allCurrentPRs), scheduleResult.RecentCount, scheduleResult.StaleCount, scheduleResult.SkippedCount)

	if len(prsToCheck) == 0 {
		log.Info().Msgf("Activity tracking: No PRs due for checking")
		return nil
	}

	// Load persisted enrichment state for all tracked PRs.
	// DetectClosedPRsUseCase.TrackPRs writes this at the end of each cycle.
	loadedSnapshots, loadErr := uc.trackingRepo.LoadAll()
	if loadErr != nil {
		log.Error().Err(loadErr).Msg("Activity tracker: failed to load prs; proceeding without prior state")
	}
	snapByURL := make(map[string]pullrequest.PullRequest, len(loadedSnapshots))
	for _, s := range loadedSnapshots {
		snapByURL[s.URL()] = *s
	}

	// Restore known enrichment state on fresh PR objects.
	// (PR objects are recreated each cycle by the GitHub adapter, so we carry
	// state forward from the persisted snapshots.)
	//
	// For pipeline status: the initial search query seeds the PR with the
	// current GitHub state via SetPipelineStatus. We OVERRIDE that with
	// the previous cycle's stored value so that UpdatePipelineStatus (called
	// during the apply phase below) can correctly detect a change vs. a no-op.
	// If we have no stored value yet, reset to Unknown so the first real status
	// from the timeline query fires the initial transition event.
	for _, pr := range prsToCheck {
		if snap, ok := snapByURL[pr.URL()]; ok {
			pr.SetHeadCommitSHA(snap.HeadCommitSHA())
			pr.SetPipelineStatus(snap.PipelineStatus())
			uc.scheduler.SeedLastChecked(pr.URL(), snap.LastActivityCheck())
		} else {
			// No stored state — reset pipeline to Unknown so the first status
			// from the batched timeline query fires a transition event.
			pr.SetPipelineStatus(pullrequest.PipelineStatusUnknown)
		}
	}

	// Fetch enrichment facts since the last check.
	// Aggregate mutation and domain event creation happen below.
	activityDataByURL, err := uc.prRepo.FetchActivities(prsToCheck, lastCheckTime)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching PR activities")
		return err
	}

	var enrichEvents []pullrequest.Event
	for _, pr := range prsToCheck {
		data, found := activityDataByURL[pr.URL()]
		if !found {
			continue
		}

		if data.HeadCommitSHA != nil {
			enrichEvents = append(enrichEvents, pr.RecordHeadCommitUpdate(*data.HeadCommitSHA)...)
		}

		if data.PipelineStatus != nil {
			enrichEvents = append(enrichEvents, pr.UpdatePipelineStatus(*data.PipelineStatus)...)
		}

		enrichEvents = append(enrichEvents, pr.AddActivities(data.Activities)...)
	}

	// Find PRs with new activity by others (filter out self-authored activities)
	// to decide which PRs should be marked unseen (asterisks in UI).
	// Must happen BEFORE building snapshots so the updated seen state is captured in Save.
	var prsWithNewActivity []*pullrequest.PullRequest
	for _, pr := range prsToCheck {
		if uc.hasActivityByOthers(pr, lastCheckTime) {
			prsWithNewActivity = append(prsWithNewActivity, pr)
		}
	}

	if len(prsWithNewActivity) > 0 {
		log.Info().Msgf("Found %d PRs with new activity", len(prsWithNewActivity))
		// Mark PRs with new activity as unseen before Save so the state is persisted.
		for _, pr := range prsWithNewActivity {
			pr.MarkAsUnseen()
		}
	} else {
		log.Info().Msgf("No new activity detected on checked PRs")
	}

	// Record the check timestamp in the scheduler. Captured once so the same
	// instant is written into every persisted snapshot below.
	checkedAt := time.Now()
	uc.scheduler.MarkCheckedAt(checkedAt, prsToCheck)

	// Persist updated enrichment state so the next cycle (and DetectClosedPRs)
	// can see the freshly-enriched values.
	//
	// We save ALL current open PRs:
	//   - PRs we just checked: use the freshly-enriched object with updated lastActivityCheck.
	//   - Stale PRs (not checked this cycle): preserve their persisted snapshot so that
	//     headCommitSHA, lastActivityCheck, pipelineStatus and seen state are not lost.
	//   - Brand-new PRs with no prior snapshot: save the live PR as-is.
	checkedByURL := make(map[string]*pullrequest.PullRequest, len(prsToCheck))
	for _, pr := range prsToCheck {
		checkedByURL[pr.URL()] = pr
	}
	updatedSnapshots := make([]*pullrequest.PullRequest, 0, len(allCurrentPRs))
	for _, pr := range allCurrentPRs {
		if checked, ok := checkedByURL[pr.URL()]; ok {
			checked.SetLastActivityCheck(checkedAt)
			updatedSnapshots = append(updatedSnapshots, checked)
		} else if snap, ok := snapByURL[pr.URL()]; ok {
			snapCopy := snap
			updatedSnapshots = append(updatedSnapshots, &snapCopy)
		} else {
			updatedSnapshots = append(updatedSnapshots, pr)
		}
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

	return nil
}

// hasActivityByOthers returns true if the PR has any activities since lastCheckTime
// that were NOT authored by the authenticated user AND are not suppressed by the
// active ignore config. Self-authored activities are domain facts but should not
// trigger unseen marking. Ignored activities are also excluded so the UI asterisk
// stays in sync with the notification suppression in NotificationAggregator.
func (uc *TrackPullRequestActivityUseCase) hasActivityByOthers(pr *pullrequest.PullRequest, since time.Time) bool {
	uc.mu.RLock()
	cfg := uc.ignoreConfig
	uc.mu.RUnlock()

	for _, activity := range pr.ActivitiesSince(since) {
		author := activity.Author().Login()
		if uc.authenticatedUser != "" && author == uc.authenticatedUser {
			continue
		}
		if cfg != nil {
			repo := pr.Repository().NameWithOwner()
			eventDetail := string(activity.Type())
			if pullrequest.ActivityIgnoreFilter(cfg, repo, pullrequest.EventActivityDetected, author, eventDetail) {
				continue
			}
		}
		return true
	}
	return false
}
