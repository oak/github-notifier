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
	trackActivityUseCase   *usecase.TrackPullRequestActivityUseCase
	updateDisplayUseCase   *usecase.UpdatePullRequestDisplayUseCase
	enableActivityTracking bool
	lastCheckTime          time.Time
}

// NewPullRequestOrchestrator creates a new orchestrator
func NewPullRequestOrchestrator(
	initializeUseCase *usecase.InitializeFirstCheckUseCase,
	checkNewPRsUseCase *usecase.CheckNewPullRequestsUseCase,
	trackActivityUseCase *usecase.TrackPullRequestActivityUseCase,
	updateDisplayUseCase *usecase.UpdatePullRequestDisplayUseCase,
	enableActivityTracking bool,
) *PullRequestOrchestrator {
	return &PullRequestOrchestrator{
		initializeUseCase:      initializeUseCase,
		checkNewPRsUseCase:     checkNewPRsUseCase,
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
	wasFirstRun, err := o.initializeUseCase.Execute(ctx)
	if err != nil {
		return err
	}

	if wasFirstRun {
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

	// Step 2: Track activity if enabled
	if o.enableActivityTracking {
		allPRs := append([]*pullrequest.PullRequest{}, result.RequestedReviewPRs...)
		allPRs = append(allPRs, result.UserCreatedPRs...)

		if err := o.trackActivityUseCase.Execute(ctx, allPRs, o.lastCheckTime); err != nil {
			log.Error().Err(err).Msg("Error tracking activity")
			// Don't return error - continue with display update
		}
	} else {
		log.Debug().Msg("Activity tracking disabled (set ENABLE_ACTIVITY_TRACKING=true to enable)")
	}

	// Step 3: Update display (after all events are emitted and state is updated)
	if err := o.updateDisplayUseCase.Execute(ctx, result.RequestedReviewPRs, result.UserCreatedPRs); err != nil {
		log.Error().Err(err).Msg("Error updating display")
		return err
	}

	// Update last check time for next iteration
	o.lastCheckTime = time.Now()

	return nil
}
