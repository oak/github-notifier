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
	log.Printf("Sending notification: New PR detected - %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())

	// Send notification with the PR from the event
	prs := []*pullrequest.PullRequest{event.PullRequest}
	if err := h.notificationPort.NotifyNewPullRequests("New PR needing review", prs); err != nil {
		log.Printf("Error sending notification for new PR: %v", err)
		return err
	}

	return nil
}

// handlePRActivityDetected sends a notification for PR activity
func (h *NotificationEventHandler) handlePRActivityDetected(event *pullrequest.PullRequestActivityDetected) error {
	log.Printf("Sending notification: New activity on PR - %s in %s (%d activities)",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner(),
		len(event.Activities))

	// Send notification with the PR from the event
	prs := []*pullrequest.PullRequest{event.PullRequest}
	if err := h.notificationPort.NotifyNewPullRequests("New activity on PR", prs); err != nil {
		log.Printf("Error sending notification for PR activity: %v", err)
		return err
	}

	return nil
}
