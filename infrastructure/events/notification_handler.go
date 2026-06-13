package events

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/oak/github-notifier/application/port"
	"github.com/oak/github-notifier/domain/pullrequest"
)

// NotificationEventHandler handles domain events by sending notifications
// Implements the EventHandler port
type NotificationEventHandler struct {
	notificationPort port.NotificationPort
	aggregator       *NotificationAggregator
}

// NewNotificationEventHandler creates a new notification event handler.
// authenticatedUser is the GitHub login of the current user; activities authored by
// this user are filtered out at the notification level (the domain records all facts,
// but we only notify about others' activity).
func NewNotificationEventHandler(notificationPort port.NotificationPort, authenticatedUser string) *NotificationEventHandler {
	handler := &NotificationEventHandler{
		notificationPort: notificationPort,
	}

	// Create aggregator with 2-second flush interval; ignore config is injected later via UpdateIgnoreConfig.
	handler.aggregator = NewNotificationAggregator(2*time.Second, handler.sendGroupedNotifications, authenticatedUser, nil)

	return handler
}

// Handle processes domain events and adds them to the aggregator
func (h *NotificationEventHandler) Handle(ctx context.Context, event pullrequest.Event) error {
	switch event.(type) {
	case *pullrequest.NewPullRequestDetected,
		*pullrequest.ActivityDetected,
		*pullrequest.ReviewStateChanged,
		*pullrequest.Merged,
		*pullrequest.Closed,
		*pullrequest.PipelineStatusChanged:
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
func (h *NotificationEventHandler) sendGroupedNotifications(notifications []*port.PRNotificationData) {
	if len(notifications) == 0 {
		return
	}

	log.Info().Msgf("Sending %d grouped PR notification(s)", len(notifications))

	if err := h.notificationPort.NotifyPullRequests(notifications); err != nil {
		log.Error().Msgf("Error sending grouped notifications: %v", err)
	}
}

// UpdateIgnoreConfig replaces the active ignore config used to filter events.
// Safe to call from any goroutine.
func (h *NotificationEventHandler) UpdateIgnoreConfig(cfg *pullrequest.IgnoreConfig) {
	if h.aggregator != nil {
		h.aggregator.UpdateIgnoreConfig(cfg)
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
