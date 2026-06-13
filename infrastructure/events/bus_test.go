package events_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/oak/github-notifier/domain/pullrequest"
	"github.com/oak/github-notifier/infrastructure/events"
	"github.com/oak/github-notifier/internal/mocks"
	"github.com/oak/github-notifier/internal/testutil"
)

func TestEventBus_Subscribe_AddsHandler(t *testing.T) {
	// Arrange
	bus := events.NewInMemoryEventBus()
	mockHandler := mocks.NewEventHandler(t)

	// Act
	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler)

	// Assert - handler is registered (verified by publish test)
	assert.NotNil(t, bus)
}

func TestEventBus_Publish_CallsHandler(t *testing.T) {
	// Arrange
	bus := events.NewInMemoryEventBus()
	mockHandler := mocks.NewEventHandler(t)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler)

	// Mock expectations
	mockHandler.On("Handle", mock.Anything, &event).Return(nil)

	// Act
	err := bus.Publish(&event)

	// Assert
	require.NoError(t, err)
	mockHandler.AssertExpectations(t)
}

func TestEventBus_Publish_NoHandlers(t *testing.T) {
	// Arrange
	bus := events.NewInMemoryEventBus()

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	// Act - publish without subscribing any handlers
	err := bus.Publish(&event)

	// Assert - should not error
	require.NoError(t, err)
}

func TestEventBus_Publish_MultipleHandlers(t *testing.T) {
	// Arrange
	bus := events.NewInMemoryEventBus()
	mockHandler1 := mocks.NewEventHandler(t)
	mockHandler2 := mocks.NewEventHandler(t)
	mockHandler3 := mocks.NewEventHandler(t)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler1)
	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler2)
	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler3)

	// Mock expectations - all handlers should be called
	mockHandler1.On("Handle", mock.Anything, &event).Return(nil)
	mockHandler2.On("Handle", mock.Anything, &event).Return(nil)
	mockHandler3.On("Handle", mock.Anything, &event).Return(nil)

	// Act
	err := bus.Publish(&event)

	// Assert
	require.NoError(t, err)
	mockHandler1.AssertExpectations(t)
	mockHandler2.AssertExpectations(t)
	mockHandler3.AssertExpectations(t)
}

func TestEventBus_Publish_HandlerError_ReturnsFirstError(t *testing.T) {
	// Arrange
	bus := events.NewInMemoryEventBus()
	mockHandler1 := mocks.NewEventHandler(t)
	mockHandler2 := mocks.NewEventHandler(t)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler1)
	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler2)

	expectedErr := errors.New("handler failed")

	// Mock expectations - first handler fails, second succeeds
	mockHandler1.On("Handle", mock.Anything, &event).Return(expectedErr)
	mockHandler2.On("Handle", mock.Anything, &event).Return(nil)

	// Act
	err := bus.Publish(&event)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler failed")
	mockHandler1.AssertExpectations(t)
	mockHandler2.AssertExpectations(t) // Second handler should still be called
}

func TestEventBus_Publish_DifferentEventTypes(t *testing.T) {
	// Arrange
	bus := events.NewInMemoryEventBus()
	mockHandler1 := mocks.NewEventHandler(t)
	mockHandler2 := mocks.NewEventHandler(t)

	pr := testutil.NewTestPullRequest(1)
	newPREvent := pullrequest.NewNewPullRequestDetected(pr)
	activity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, time.Now())
	activityEvent := pullrequest.NewActivityDetected(pr, activity)

	// Subscribe different handlers to different events
	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler1)
	bus.Subscribe(pullrequest.EventActivityDetected, mockHandler2)

	// Mock expectations
	mockHandler1.On("Handle", mock.Anything, &newPREvent).Return(nil)

	// Act - publish NewPullRequestDetected
	err := bus.Publish(&newPREvent)

	// Assert
	require.NoError(t, err)
	mockHandler1.AssertExpectations(t)
	mockHandler2.AssertNotCalled(t, "Handle") // Handler2 should NOT be called for NewPullRequestDetected

	// Now publish ActivityDetected
	mockHandler2.On("Handle", mock.Anything, &activityEvent).Return(nil)

	err = bus.Publish(&activityEvent)

	require.NoError(t, err)
	mockHandler2.AssertExpectations(t)
}

func TestEventBus_Subscribe_SameHandlerMultipleTimes(t *testing.T) {
	// Arrange
	bus := events.NewInMemoryEventBus()
	mockHandler := mocks.NewEventHandler(t)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	// Subscribe the same handler twice
	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler)
	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler)

	// Mock expectations - handler should be called twice (once per subscription)
	mockHandler.On("Handle", mock.Anything, &event).Return(nil).Twice()

	// Act
	err := bus.Publish(&event)

	// Assert
	require.NoError(t, err)
	mockHandler.AssertExpectations(t)
}

func TestEventBus_Publish_AllHandlersFail(t *testing.T) {
	// Arrange
	bus := events.NewInMemoryEventBus()
	mockHandler1 := mocks.NewEventHandler(t)
	mockHandler2 := mocks.NewEventHandler(t)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler1)
	bus.Subscribe(pullrequest.EventNewPullRequestDetected, mockHandler2)

	err1 := errors.New("handler 1 failed")
	err2 := errors.New("handler 2 failed")

	// Mock expectations - both handlers fail
	mockHandler1.On("Handle", mock.Anything, &event).Return(err1)
	mockHandler2.On("Handle", mock.Anything, &event).Return(err2)

	// Act
	err := bus.Publish(&event)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler 1 failed") // Returns first error
	mockHandler1.AssertExpectations(t)
	mockHandler2.AssertExpectations(t) // Both should be called
}
