package notification

import (
	"github.com/rs/zerolog/log"

	"github.com/oak/github-notifier/application/port"
)

// CompositeAdapter implements port.NotificationPort by delegating to multiple adapters
type CompositeAdapter struct {
	adapters []port.NotificationPort
}

// NewCompositeAdapter creates a new composite adapter
func NewCompositeAdapter(adapters ...port.NotificationPort) port.NotificationPort {
	return &CompositeAdapter{
		adapters: adapters,
	}
}

// NotifyPullRequests sends grouped notifications to all configured adapters
func (c *CompositeAdapter) NotifyPullRequests(notifications []*port.PRNotificationData) error {
	if len(notifications) == 0 {
		return nil
	}

	var firstError error
	for _, adapter := range c.adapters {
		if err := adapter.NotifyPullRequests(notifications); err != nil {
			log.Error().Msgf("Notification adapter failed: %v", err)
			if firstError == nil {
				firstError = err
			}
			// Continue sending to other adapters even if one fails
		}
	}

	return firstError
}

// NotifyMessage sends a simple text notification to all configured adapters
func (c *CompositeAdapter) NotifyMessage(title, message string) error {
	var firstError error
	for _, adapter := range c.adapters {
		if err := adapter.NotifyMessage(title, message); err != nil {
			log.Error().Msgf("Notification adapter failed: %v", err)
			if firstError == nil {
				firstError = err
			}
		}
	}
	return firstError
}

// SupportsClickActions returns true if any adapter supports click actions
func (c *CompositeAdapter) SupportsClickActions() bool {
	for _, adapter := range c.adapters {
		if adapter.SupportsClickActions() {
			return true
		}
	}
	return false
}
