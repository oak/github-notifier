package events

import (
	"context"
	"log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// NotificationEventHandler handles domain events by sending notifications
// Implements the EventHandler port
type NotificationEventHandler struct {
	notificationPort port.NotificationPort
}

// NewNotificationEventHandler creates a new notification event handler
func NewNotificationEventHandler(notificationPort port.NotificationPort) *NotificationEventHandler {
	return &NotificationEventHandler{
		notificationPort: notificationPort,
	}
}

// Handle processes domain events and sends appropriate notifications
func (h *NotificationEventHandler) Handle(ctx context.Context, event pullrequest.Event) error {
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

// handleNewPRDetected sends a notification for newly detected PRs
func (h *NotificationEventHandler) handleNewPRDetected(event *pullrequest.NewPullRequestDetected) error {
	// For now, we'll need to reconstruct a PR from the event
	// In a real system, you might pass the full PR or fetch it
	log.Printf("Notification: New PR detected - %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())

	// Note: The current NotificationPort expects a slice of PRs
	// In Phase 3, we'll refactor the use cases to provide the full PR list
	// For now, this handler just logs
	return nil
}

// handlePRActivityDetected sends a notification for PR activity
func (h *NotificationEventHandler) handlePRActivityDetected(event *pullrequest.PullRequestActivityDetected) error {
	log.Printf("Notification: New activity on PR - %s in %s (%d activities)",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner(),
		len(event.Activities))

	// Note: Same as above - will be properly implemented in Phase 3
	return nil
}
