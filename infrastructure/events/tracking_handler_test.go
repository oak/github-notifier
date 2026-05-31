package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/events"
	"github.com/oak3/github-notifier/internal/mocks"
	"github.com/oak3/github-notifier/internal/testutil"
)

func TestTrackingHandler_HandleNewPRDetected_Success(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	// Handler currently just logs, no tracking service calls expected
	mockPRTrackingRepo.AssertNotCalled(t, "MarkAsSeen")
}

func TestTrackingHandler_HandleActivityDetected_Success(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)

	event := pullrequest.NewActivityDetected(pr, activity)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	// Handler currently just logs, no tracking service calls expected
	mockPRTrackingRepo.AssertNotCalled(t, "UnmarkAsSeen")
}

func TestTrackingHandler_HandleUnknownEvent_Ignored(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	// Create a mock event type (not a real event, just for testing)
	type UnknownEvent struct {
		pullrequest.Event
	}
	unknownEvent := &UnknownEvent{}

	// Act
	err := handler.Handle(context.Background(), unknownEvent)

	// Assert
	require.NoError(t, err)
	// No tracking service calls should be made for unknown event types
	mockPRTrackingRepo.AssertNotCalled(t, "MarkAsSeen")
	mockPRTrackingRepo.AssertNotCalled(t, "UnmarkAsSeen")
}

func TestTrackingHandler_HandleMultipleNewPRs(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr1 := testutil.NewTestPullRequest(1)
	pr2 := testutil.NewTestPullRequest(2)

	event1 := pullrequest.NewNewPullRequestDetected(pr1)
	event2 := pullrequest.NewNewPullRequestDetected(pr2)

	// Act
	err1 := handler.Handle(context.Background(), &event1)
	err2 := handler.Handle(context.Background(), &event2)

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	// Both events should be handled successfully
}

func TestTrackingHandler_HandleMultipleActivities(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr := testutil.NewTestPullRequest(1)
	activity1 := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)
	activity2 := testutil.NewTestActivity(
		pullrequest.ActivityTypeReview,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)

	event1 := pullrequest.NewActivityDetected(pr, activity1)
	event2 := pullrequest.NewActivityDetected(pr, activity2)

	// Act
	err := handler.Handle(context.Background(), &event1)
	require.NoError(t, err)
	err = handler.Handle(context.Background(), &event2)

	// Assert
	require.NoError(t, err)
	// Event with multiple activities should be handled successfully
}

func TestTrackingHandler_HandleMixedEvents(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr1 := testutil.NewTestPullRequest(1)
	pr2 := testutil.NewTestPullRequest(2)

	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr2.URL(), pr2.Number()),
	)

	newPREvent := pullrequest.NewNewPullRequestDetected(pr1)
	activityEvent := pullrequest.NewActivityDetected(pr2, activity)

	// Act
	err1 := handler.Handle(context.Background(), &newPREvent)
	err2 := handler.Handle(context.Background(), &activityEvent)

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	// Both event types should be handled successfully
}

func TestTrackingHandler_HandlePRMerged(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewMerged(pr)

	// Merged event should mark PR unseen and persist via Update.
	mockPRTrackingRepo.On("Update", pr).Return(nil).Once()

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	mockPRTrackingRepo.AssertExpectations(t)
}

func TestTrackingHandler_HandlePRClosed(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewClosed(pr)

	// Closed event should mark PR unseen and persist via Update.
	mockPRTrackingRepo.On("Update", pr).Return(nil).Once()

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	mockPRTrackingRepo.AssertExpectations(t)
}

func TestTrackingHandler_HandleStatusChanged(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewStatusChanged(pr, pullrequest.StatusOpen, pullrequest.StatusMerged)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
}

func TestTrackingHandler_HandlePipelineStatusChanged(t *testing.T) {
	// Arrange
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)

	handler := events.NewTrackingEventHandler(mockPRTrackingRepo)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewPipelineStatusChanged(pr, pullrequest.PipelineStatusRunning, pullrequest.PipelineStatusSuccess)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert - should handle without error, just logs
	require.NoError(t, err)
	// No tracking service calls expected (pipeline events are log-only)
	mockPRTrackingRepo.AssertNotCalled(t, "MarkAsSeen")
	mockPRTrackingRepo.AssertNotCalled(t, "UnmarkAsSeen")
}
