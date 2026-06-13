package events

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/oak/github-notifier/application/port"
	"github.com/oak/github-notifier/domain/pullrequest"
)

// InMemoryEventBus is a simple in-memory event dispatcher
// Implements the EventPublisher port
type InMemoryEventBus struct {
	handlers map[string][]port.EventHandler
}

// NewInMemoryEventBus creates a new event bus
func NewInMemoryEventBus() *InMemoryEventBus {
	return &InMemoryEventBus{
		handlers: make(map[string][]port.EventHandler),
	}
}

// Subscribe registers an event handler for a specific event type
// eventType should be the concrete type name (e.g., "NewPullRequestDetected")
func (bus *InMemoryEventBus) Subscribe(eventType string, handler port.EventHandler) {
	bus.handlers[eventType] = append(bus.handlers[eventType], handler)
	log.Info().Msgf("Event handler registered for: %s by %T", eventType, handler)
}

// Publish dispatches an event to all registered handlers for that event type
// Implements the EventPublisher port interface
func (bus *InMemoryEventBus) Publish(event pullrequest.Event) error {
	eventType := event.Name()

	handlers, exists := bus.handlers[eventType]
	if !exists || len(handlers) == 0 {
		log.Info().Msgf("No handlers registered for event: %s", eventType)
		return nil
	}

	log.Info().Msgf("Publishing event: %s to %d handler(s)", eventType, len(handlers))

	ctx := context.Background()
	var errs []error

	for _, handler := range handlers {
		if err := handler.Handle(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("handler failed for %s: %w", eventType, err))
		}
	}

	if len(errs) > 0 {
		// Return first error but log all
		for _, err := range errs {
			log.Error().Msgf("Event handler error: %v", err)
		}
		return errs[0]
	}

	return nil
}
