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
	lastCheckTime          time.Time
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
		lastCheckTime:          time.Now(),
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
		// Seed the activity tracker with the current state of all existing PRs so
		// that the first regular check does not fire spurious PipelineStatusChanged
		// events for every PR that already has a known pipeline status.
		o.trackActivityUseCase.SeedKnownState(firstRunPRs)
		log.Info().Msg("First run complete - all existing PRs marked as seen")
		return nil
	}

	// Not first run - execute regular check
	log.Info().Msg("Existing state detected - checking for updates")
	return o.ExecuteRegularCheck(ctx)
}

// ExecuteRegularCheck runs a regular periodic check for PR updates
func (o *PullRequestOrchestrator) ExecuteRegularCheck(ctx context.Context) error {
	// Step 1: Fetch and check for new PRs (emits events)
	result, err := o.checkNewPRsUseCase.Execute(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error checking for new PRs")
		return err
	}

	// Step 2: Detect merged/closed PRs
	// Combine all current PRs to compare against tracked state
	allCurrentPRs := append([]*pullrequest.PullRequest{}, result.RequestedReviewPRs...)
	allCurrentPRs = append(allCurrentPRs, result.UserCreatedPRs...)

	if err := o.detectClosedPRsUseCase.Execute(ctx, allCurrentPRs); err != nil {
		log.Error().Err(err).Msg("Error detecting closed PRs")
		// Don't return error — continue with other steps
	}

	// Update tracked PRs for next cycle's comparison
	o.detectClosedPRsUseCase.TrackPRs(allCurrentPRs)

	// Step 3: Track activity if enabled
	if o.enableActivityTracking {
		if err := o.trackActivityUseCase.Execute(ctx, allCurrentPRs, o.lastCheckTime); err != nil {
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

	// Update last check time for next iteration
	o.lastCheckTime = time.Now()

	return nil
}
