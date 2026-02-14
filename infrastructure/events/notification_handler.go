package events

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// NotificationEventHandler handles domain events by sending notifications
// Implements the EventHandler port
type NotificationEventHandler struct {
	notificationPort port.NotificationPort
	aggregator       *NotificationAggregator
}

// NewNotificationEventHandler creates a new notification event handler
func NewNotificationEventHandler(notificationPort port.NotificationPort) *NotificationEventHandler {
	handler := &NotificationEventHandler{
		notificationPort: notificationPort,
	}

	// Create aggregator with 2-second flush interval
	handler.aggregator = NewNotificationAggregator(2*time.Second, handler.sendGroupedNotifications)

	return handler
}

// Handle processes domain events and adds them to the aggregator
func (h *NotificationEventHandler) Handle(ctx context.Context, event pullrequest.Event) error {
	switch event.(type) {
	case *pullrequest.NewPullRequestDetected,
		*pullrequest.ActivityDetected,
		*pullrequest.Merged,
		*pullrequest.Closed:
		// Add event to aggregator for batching
		h.aggregator.AddEvent(event)
		return nil

	case *pullrequest.StatusChanged:
		// Status changes are already handled by specific events (merged, closed)
		return nil

	default:
		// Ignore other event types
		return nil
	}
}

// sendGroupedNotifications sends the aggregated notifications
func (h *NotificationEventHandler) sendGroupedNotifications(notifications []*PRNotification) {
	if len(notifications) == 0 {
		return
	}

	log.Info().Msgf("Sending %d grouped PR notification(s)", len(notifications))

	// Convert to port notification data
	portNotifications := make([]*port.PRNotificationData, 0, len(notifications))
	for _, notification := range notifications {
		portNotification := &port.PRNotificationData{
			PullRequest:   notification.PullRequest,
			IsNew:         notification.IsNew,
			Activities:    make([]port.ActivityInfo, len(notification.Activities)),
			StatusChanges: make([]port.StatusChange, len(notification.StatusChanges)),
		}

		// Convert activities
		for i, activity := range notification.Activities {
			portNotification.Activities[i] = port.ActivityInfo{
				Type:  activity.Type,
				Count: activity.Count,
			}
		}

		// Convert status changes
		for i, statusChange := range notification.StatusChanges {
			portNotification.StatusChanges[i] = port.StatusChange{
				EventType: statusChange.EventType,
			}
		}

		portNotifications = append(portNotifications, portNotification)
	}

	// Send the grouped notifications
	if err := h.notificationPort.NotifyPullRequests(portNotifications); err != nil {
		log.Error().Msgf("Error sending grouped notifications: %v", err)
	}
}

// Stop stops the handler and flushes any pending notifications
func (h *NotificationEventHandler) Stop() {
	if h.aggregator != nil {
		h.aggregator.Stop()
	}
}

// Flush immediately flushes any pending notifications without stopping the handler
func (h *NotificationEventHandler) Flush() {
	if h.aggregator != nil {
		h.aggregator.Flush()
	}
}
