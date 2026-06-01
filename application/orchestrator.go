package application

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// PullRequestOrchestrator coordinates multiple use cases to check for PR updates
// This is the main entry point for periodic PR checking
type PullRequestOrchestrator struct {
	initializeUseCase      *usecase.InitializeFirstCheckUseCase
	checkNewPRsUseCase     *usecase.CheckNewPullRequestsUseCase
	detectClosedPRsUseCase *usecase.DetectClosedPullRequestsUseCase
	trackActivityUseCase   *usecase.TrackPullRequestActivityUseCase
	updateDisplayUseCase   *usecase.UpdatePullRequestDisplayUseCase
	enableActivityTracking bool
	checkCycleState        usecase.CheckCycleState
}

// NewPullRequestOrchestrator creates a new orchestrator
func NewPullRequestOrchestrator(
	initializeUseCase *usecase.InitializeFirstCheckUseCase,
	checkNewPRsUseCase *usecase.CheckNewPullRequestsUseCase,
	detectClosedPRsUseCase *usecase.DetectClosedPullRequestsUseCase,
	trackActivityUseCase *usecase.TrackPullRequestActivityUseCase,
	updateDisplayUseCase *usecase.UpdatePullRequestDisplayUseCase,
	enableActivityTracking bool,
) *PullRequestOrchestrator {
	return &PullRequestOrchestrator{
		initializeUseCase:      initializeUseCase,
		checkNewPRsUseCase:     checkNewPRsUseCase,
		detectClosedPRsUseCase: detectClosedPRsUseCase,
		trackActivityUseCase:   trackActivityUseCase,
		updateDisplayUseCase:   updateDisplayUseCase,
		enableActivityTracking: enableActivityTracking,
		checkCycleState:        usecase.NewCheckCycleState(),
	}
}

// ExecuteInitialCheck runs the initial check on application startup
// Returns true if this was the first run ever
func (o *PullRequestOrchestrator) ExecuteInitialCheck(ctx context.Context) error {
	log.Info().Msg("Performing initial PR check")

	// Try first-run initialization
	wasFirstRun, firstRunPRs, err := o.initializeUseCase.Execute(ctx)
	if err != nil {
		return err
	}

	if wasFirstRun {
		log.Info().Msg("First run complete - all existing PRs marked as seen")
		// Seed the tracking repo with initial PR state (including pipeline status)
		// so the first regular check can correctly detect changes from baseline.
		o.detectClosedPRsUseCase.TrackPRs(firstRunPRs)
		return nil
	}

	// Not first run - execute regular check
	log.Info().Msg("Existing state detected - checking for updates")
	return o.ExecuteRegularCheck(ctx, time.Now())
}

// ExecuteRegularCheck runs a regular periodic check for PR updates.
// lastCheckTime is the timestamp captured before the previous cycle ran;
// it is owned and threaded by the caller (e.g. the polling goroutine in main).
func (o *PullRequestOrchestrator) ExecuteRegularCheck(ctx context.Context, lastCheckTime time.Time) error {
	// Step 1: Fetch and check for new PRs (emits events)
	result, updatedState, err := o.checkNewPRsUseCase.Execute(ctx, o.checkCycleState)
	if err != nil {
		log.Error().Err(err).Msg("Error checking for new PRs")
		return err
	}
	o.checkCycleState = updatedState

	// Step 2: Detect merged/closed PRs
	// Combine all current PRs to compare against tracked state.
	// Deduplicate by URL: a PR can appear in both RequestedReviewPRs and
	// UserCreatedPRs (e.g. author is also a reviewer), and duplicate entries
	// cause double pipeline-status events and corrupt snapshot state.
	seen := make(map[string]bool, len(result.RequestedReviewPRs)+len(result.UserCreatedPRs))
	allCurrentPRs := make([]*pullrequest.PullRequest, 0, len(result.RequestedReviewPRs)+len(result.UserCreatedPRs))
	for _, pr := range append(result.RequestedReviewPRs, result.UserCreatedPRs...) {
		if !seen[pr.URL()] {
			seen[pr.URL()] = true
			allCurrentPRs = append(allCurrentPRs, pr)
		}
	}

	closedMergedURLs, err := o.detectClosedPRsUseCase.Execute(ctx, allCurrentPRs)
	if err != nil {
		log.Error().Err(err).Msg("Error detecting closed PRs")
		// Don't return error — continue with other steps
	} else if len(closedMergedURLs) > 0 {
		// Clean up cycle state to free memory (avoid unbounded growth)
		// Remove closed/merged PRs from KnownPRs and KnownReviews
		for _, url := range closedMergedURLs {
			delete(o.checkCycleState.KnownPRs, url)
			delete(o.checkCycleState.KnownReviews, url)
		}
		log.Debug().Msgf("Pruned %d closed/merged PRs from cycle state", len(closedMergedURLs))
	}

	// Update tracked PRs for next cycle's comparison
	o.detectClosedPRsUseCase.TrackPRs(allCurrentPRs)

	// Step 3: Track activity if enabled
	if o.enableActivityTracking {
		if err := o.trackActivityUseCase.Execute(ctx, allCurrentPRs, lastCheckTime); err != nil {
			log.Error().Err(err).Msg("Error tracking activity")
			// Don't return error - continue with display update
		}
	} else {
		log.Debug().Msg("Activity tracking disabled (set ENABLE_ACTIVITY_TRACKING=true to enable)")
	}

	// Step 4: Update display (after all events are emitted and state is updated)
	if err := o.updateDisplayUseCase.Execute(ctx, result.RequestedReviewPRs, result.UserCreatedPRs); err != nil {
		log.Error().Err(err).Msg("Error updating display")
		return err
	}

	return nil
}
