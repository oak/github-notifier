package events

import (
	"context"
	"log"

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

	case *pullrequest.PullRequestActivityDetected:
		return h.handlePRActivityDetected(e)

	case *pullrequest.PullRequestMerged:
		return h.handlePRMerged(e)

	case *pullrequest.PullRequestClosed:
		return h.handlePRClosed(e)

	case *pullrequest.PullRequestStatusChanged:
		return h.handleStatusChanged(e)

	default:
		// Ignore other event types
		return nil
	}
}

// handleNewPRDetected marks newly detected PRs as seen after notification
func (h *TrackingEventHandler) handleNewPRDetected(event *pullrequest.NewPullRequestDetected) error {
	// The PR will be marked as seen by the use case after emitting the event
	// This handler could be used for additional tracking logic if needed
	log.Printf("Tracking: New PR detected - %s", event.PullRequestID.URL())
	return nil
}

// handlePRActivityDetected handles activity detection events
func (h *TrackingEventHandler) handlePRActivityDetected(event *pullrequest.PullRequestActivityDetected) error {
	// When activity is detected, the PR should remain unseen to show asterisks
	// This handler could implement additional activity tracking logic
	log.Printf("Tracking: Activity detected on PR - %s (%d activities)",
		event.PullRequestID.URL(),
		len(event.Activities))
	return nil
}

// handlePRMerged handles PR merged events
func (h *TrackingEventHandler) handlePRMerged(event *pullrequest.PullRequestMerged) error {
	log.Printf("Tracking: PR merged - %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())
	// Could remove from tracking or mark differently
	return nil
}

// handlePRClosed handles PR closed events
func (h *TrackingEventHandler) handlePRClosed(event *pullrequest.PullRequestClosed) error {
	log.Printf("Tracking: PR closed - %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())
	// Could remove from tracking or mark differently
	return nil
}

// handleStatusChanged handles PR status change events
func (h *TrackingEventHandler) handleStatusChanged(event *pullrequest.PullRequestStatusChanged) error {
	log.Printf("Tracking: PR status changed from %v to %v - %s",
		event.OldStatus,
		event.NewStatus,
		event.PullRequestID.URL())
	return nil
}
