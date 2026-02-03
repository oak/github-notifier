package events

import (
	"context"

	"github.com/rs/zerolog/log"

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

	case *pullrequest.ActivityDetected:
		return h.handlePRActivityDetected(e)

	case *pullrequest.Merged:
		return h.handlePRMerged(e)

	case *pullrequest.Closed:
		return h.handlePRClosed(e)

	case *pullrequest.StatusChanged:
		// Status changes are already handled by specific events (merged, closed)
		return nil

	default:
		// Ignore other event types
		return nil
	}
}

// handleNewPRDetected sends a notification for newly detected PRs
func (h *NotificationEventHandler) handleNewPRDetected(event *pullrequest.NewPullRequestDetected) error {
	log.Info().Msgf("Sending notification: New PR detected - %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())

	// Send notification with the PR from the event
	prs := []*pullrequest.PullRequest{event.PullRequest}
	if err := h.notificationPort.NotifyNewPullRequests("New PR needing review", prs); err != nil {
		log.Error().Msgf("Error sending notification for new PR: %v", err)
		return err
	}

	return nil
}

// handlePRActivityDetected sends a notification for PR activity
func (h *NotificationEventHandler) handlePRActivityDetected(event *pullrequest.ActivityDetected) error {
	log.Info().Msgf("Sending notification: New activity on PR - %s in %s (%d activities)",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner(),
		len(event.Activities))

	// Determine notification title based on activity types
	title := h.getNotificationTitle(event.Activities)

	// Send notification with the PR from the event
	prs := []*pullrequest.PullRequest{event.PullRequest}
	if err := h.notificationPort.NotifyNewPullRequests(title, prs); err != nil {
		log.Error().Msgf("Error sending notification for PR activity: %v", err)
		return err
	}

	return nil
}

// getNotificationTitle determines the appropriate notification title based on activity types
// Priority: push > review > comment > other
func (h *NotificationEventHandler) getNotificationTitle(activities []*pullrequest.Activity) string {
	hasPush := false
	hasReview := false
	hasComment := false

	// Scan all activities to determine priority
	for _, activity := range activities {
		switch activity.Type() {
		case pullrequest.ActivityTypePush:
			hasPush = true
		case pullrequest.ActivityTypeReview:
			hasReview = true
		case pullrequest.ActivityTypeComment:
			hasComment = true
		}
	}

	// Return based on priority
	if hasPush {
		return "New commits pushed to PR"
	}
	if hasReview {
		return "New review on PR"
	}
	if hasComment {
		return "New comment on PR"
	}

	// Default fallback
	return "New activity on PR"
}

// handlePRMerged sends a notification when a PR is merged
func (h *NotificationEventHandler) handlePRMerged(event *pullrequest.Merged) error {
	log.Info().Msgf("PR merged: %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())
	// Could send a notification if desired, but typically merges don't need notifications
	return nil
}

// handlePRClosed sends a notification when a PR is closed
func (h *NotificationEventHandler) handlePRClosed(event *pullrequest.Closed) error {
	log.Info().Msgf("PR closed: %s in %s",
		event.PullRequestID.URL(),
		event.Repository.NameWithOwner())
	// Could send a notification if desired, but typically closes don't need notifications
	return nil
}
