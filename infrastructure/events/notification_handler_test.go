package events_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/events"
	"github.com/oak3/github-notifier/internal/mocks"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNotificationHandler_HandleNewPRDetected_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	// Mock expectations
	mockNotificationPort.On("NotifyNewPullRequests", "New PR needing review", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		return len(prs) == 1 && prs[0] == pr
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleNewPRDetected_NotificationError(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	expectedErr := errors.New("notification service unavailable")

	// Mock expectations
	mockNotificationPort.On("NotifyNewPullRequests", "New PR needing review", mock.AnythingOfType("[]*pullrequest.PullRequest")).Return(expectedErr)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleActivityDetected_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)
	pr.AddActivities([]*pullrequest.Activity{activity})

	event := pullrequest.NewPullRequestActivityDetected(pr)

	// Mock expectations
	mockNotificationPort.On("NotifyNewPullRequests", "New activity on PR", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		return len(prs) == 1 && prs[0] == pr
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleActivityDetected_NotificationError(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)
	pr.AddActivities([]*pullrequest.Activity{activity})

	event := pullrequest.NewPullRequestActivityDetected(pr)

	expectedErr := errors.New("notification failed")

	// Mock expectations
	mockNotificationPort.On("NotifyNewPullRequests", "New activity on PR", mock.AnythingOfType("[]*pullrequest.PullRequest")).Return(expectedErr)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleUnknownEvent_Ignored(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	// Create a mock event type (not a real event, just for testing)
	type UnknownEvent struct {
		pullrequest.Event
	}
	unknownEvent := &UnknownEvent{}

	// Act
	err := handler.Handle(context.Background(), unknownEvent)

	// Assert
	require.NoError(t, err)
	// No notification should be sent for unknown event types
	mockNotificationPort.AssertNotCalled(t, "NotifyNewPullRequests")
}

func TestNotificationHandler_HandleMultipleActivities(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

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
	pr.AddActivities([]*pullrequest.Activity{activity1, activity2})

	event := pullrequest.NewPullRequestActivityDetected(pr)

	// Mock expectations
	mockNotificationPort.On("NotifyNewPullRequests", "New activity on PR", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		// Should send the PR with both activities
		return len(prs) == 1 && prs[0] == pr && len(prs[0].Activities()) == 2
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleNewPR_WithDraftStatus(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	pr := testutil.NewTestPullRequest(1, testutil.WithDraft(true))
	event := pullrequest.NewNewPullRequestDetected(pr)

	// Mock expectations - should still send notification even for drafts
	mockNotificationPort.On("NotifyNewPullRequests", "New PR needing review", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		return len(prs) == 1 && prs[0].IsDraft()
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandlePRMerged_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewPullRequestMerged(pr)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	// No notification should be sent, so no mock expectations needed
}

func TestNotificationHandler_HandlePRClosed_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewPullRequestClosed(pr)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	// No notification should be sent, so no mock expectations needed
}

func TestNotificationHandler_HandleStatusChanged_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort)

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewPullRequestStatusChanged(pr, pullrequest.StatusOpen, pullrequest.StatusMerged)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	// Status changes are handled by specific events, so no notification expected
}
