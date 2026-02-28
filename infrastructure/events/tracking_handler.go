package events

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// TrackingEventHandler handles domain events by updating tracking state
// Implements the EventHandler port
type TrackingEventHandler struct {
	trackingService *pullrequest.TrackingService
}

// NewTrackingEventHandler creates a new tracking event handler
func NewTrackingEventHandler(trackingService *pullrequest.TrackingService) *TrackingEventHandler {
	return &TrackingEventHandler{
		trackingService: trackingService,
	}
}

// Handle processes domain events and updates tracking state accordingly
func (h *TrackingEventHandler) Handle(ctx context.Context, event pullrequest.Event) error {
	switch e := event.(type) {
	case *pullrequest.NewPullRequestDetected:
		return h.handleNewPRDetected(e)

	case *pullrequest.ActivityDetected:
		return h.handlePRActivityDetected(e)

	case *pullrequest.ReviewStateChanged:
		return h.handleReviewStateChanged(e)

	case *pullrequest.Merged:
		return h.handlePRMerged(e)

	case *pullrequest.Closed:
		return h.handlePRClosed(e)

	case *pullrequest.StatusChanged:
		return h.handleStatusChanged(e)

	case *pullrequest.PipelineStatusChanged:
		return h.handlePipelineStatusChanged(e)

	default:
		// Ignore other event types
		return nil
	}
}

// handleNewPRDetected marks newly detected PRs as seen after notification
func (h *TrackingEventHandler) handleNewPRDetected(event *pullrequest.NewPullRequestDetected) error {
	// The PR will be marked as seen by the use case after emitting the event
	// This handler could be used for additional tracking logic if needed
	log.Info().Msgf("Tracking: New PR detected - %s", event.PullRequestID.URL())
	return nil
}

// handlePRActivityDetected handles activity detection events
func (h *TrackingEventHandler) handlePRActivityDetected(event *pullrequest.ActivityDetected) error {
	// When activity is detected, the PR should remain unseen to show asterisks
	// This handler could implement additional activity tracking logic
	log.Info().Msgf("Tracking: Activity detected on PR - %s (%s by %s)",
		event.PullRequestID.URL(),
		event.Activity.Type(),
		event.Activity.Author().Login())
	return nil
}

// handlePRMerged handles PR merged events
func (h *TrackingEventHandler) handlePRMerged(event *pullrequest.Merged) error {
	log.Info().Msgf("Tracking: PR merged - %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())
	// Remove from tracking so it doesn't trigger repeated notifications
	h.trackingService.RemoveSeen(event.PullRequestID)
	return nil
}

// handlePRClosed handles PR closed events
func (h *TrackingEventHandler) handlePRClosed(event *pullrequest.Closed) error {
	log.Info().Msgf("Tracking: PR closed - %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())
	// Remove from tracking so it doesn't trigger repeated notifications
	h.trackingService.RemoveSeen(event.PullRequestID)
	return nil
}

// handleStatusChanged handles PR status change events
func (h *TrackingEventHandler) handleStatusChanged(event *pullrequest.StatusChanged) error {
	log.Info().Msgf("Tracking: PR status changed from %v to %v - %s",
		event.OldStatus,
		event.NewStatus,
		event.PullRequestID.URL())
	return nil
}

// handleReviewStateChanged handles review state change events
func (h *TrackingEventHandler) handleReviewStateChanged(event *pullrequest.ReviewStateChanged) error {
	log.Info().Msgf("Tracking: Review state changed on PR - %s (%s %s)",
		event.PullRequestID.URL(),
		event.Reviewer.Login(),
		event.State.Label())
	return nil
}

// handlePipelineStatusChanged handles pipeline status change events
func (h *TrackingEventHandler) handlePipelineStatusChanged(event *pullrequest.PipelineStatusChanged) error {
	log.Info().Msgf("Tracking: Pipeline status changed %s → %s on PR - %s",
		event.OldStatus,
		event.NewStatus,
		event.PullRequestID.URL())
	return nil
}
