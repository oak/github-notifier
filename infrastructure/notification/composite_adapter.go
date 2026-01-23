package notification

import (
	"log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
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

// NotifyNewPullRequests sends notifications to all configured adapters
func (c *CompositeAdapter) NotifyNewPullRequests(title string, prs []*pullrequest.PullRequest) error {
	if len(prs) == 0 {
		return nil
	}

	var firstError error
	for _, adapter := range c.adapters {
		if err := adapter.NotifyNewPullRequests(title, prs); err != nil {
			log.Printf("Notification adapter failed: %v", err)
			if firstError == nil {
				firstError = err
			}
			// Continue sending to other adapters even if one fails
		}
	}

	return firstError
}
