package port

import (
	"context"

	"github.com/oak/github-notifier/domain/pullrequest"
)

// EventHandler is the port for handling domain events
// Implementations react to events by sending notifications, updating state, etc.
type EventHandler interface {
	// Handle processes a domain event
	// Returns error if handling fails
	Handle(ctx context.Context, event pullrequest.Event) error
}
