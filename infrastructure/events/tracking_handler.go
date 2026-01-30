package events

import (
	"context"
	"log"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/domain/tracking"
)

// TrackingEventHandler handles domain events by updating tracking state
// Implements the EventHandler port
type TrackingEventHandler struct {
	trackingService tracking.Service
}

// NewTrackingEventHandler creates a new tracking event handler
func NewTrackingEventHandler(trackingService tracking.Service) *TrackingEventHandler {
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
