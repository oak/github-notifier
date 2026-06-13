package port

import "github.com/oak/github-notifier/domain/pullrequest"

// EventPublisher is the port for publishing domain events
// Implementations can dispatch events to handlers, persist them, or send to external systems
type EventPublisher interface {
	// Publish dispatches a domain event to all registered handlers
	Publish(event pullrequest.Event) error
}
