package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/oak/github-notifier/application/port"
	"github.com/oak/github-notifier/domain/pullrequest"
	"github.com/oak/github-notifier/infrastructure/events"
	"github.com/oak/github-notifier/internal/mocks"
	"github.com/oak/github-notifier/internal/testutil"
)

func TestNotificationHandler_HandleNewPRDetected_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	// Mock expectations - now expects NotifyPullRequests with grouped data
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		if len(notifications) != 1 {
			return false
		}
		notif := notifications[0]
		return notif.IsNew && notif.PullRequest == pr && len(notif.Activities) == 0
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)
	require.NoError(t, err)

	// Wait for aggregator to flush (2 seconds + buffer)
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleActivityDetected_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)

	event := pullrequest.NewActivityDetected(pr, activity)

	// Mock expectations - should group activities
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		if len(notifications) != 1 {
			return false
		}
		notif := notifications[0]
		return !notif.IsNew &&
			notif.PullRequest == pr &&
			len(notif.Activities) == 1 &&
			notif.Activities[0].Type == pullrequest.ActivityTypeComment &&
			notif.Activities[0].Count == 1
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)
	require.NoError(t, err)

	// Wait for aggregator to flush
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleMultipleEvents_GroupedBySamePR(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)

	// First event: new PR
	newPREvent := pullrequest.NewNewPullRequestDetected(pr)

	// Second event: activity on same PR
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)
	activityEvent := pullrequest.NewActivityDetected(pr, activity)

	// Mock expectations - should group into ONE notification
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		if len(notifications) != 1 {
			return false
		}
		notif := notifications[0]
		// Should be marked as new AND have activity
		return notif.IsNew &&
			len(notif.Activities) == 1 &&
			notif.Activities[0].Type == pullrequest.ActivityTypeComment
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &newPREvent)
	require.NoError(t, err)

	err = handler.Handle(context.Background(), &activityEvent)
	require.NoError(t, err)

	// Wait for aggregator to flush
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
	mockNotificationPort.AssertNumberOfCalls(t, "NotifyPullRequests", 1) // Only one call!
}

func TestNotificationHandler_HandleMultipleActivitiesSamePR(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)

	// Create two separate activity events (each AddActivity raises its own event)
	comment := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)
	review := testutil.NewTestActivity(
		pullrequest.ActivityTypeReview,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)

	commentEvent := pullrequest.NewActivityDetected(pr, comment)
	reviewEvent := pullrequest.NewActivityDetected(pr, review)

	// Mock expectations - should group activities by type
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		if len(notifications) != 1 {
			return false
		}
		notif := notifications[0]
		// Should have 2 activity types
		if len(notif.Activities) != 2 {
			return false
		}
		// Check both types are present
		hasComment := false
		hasReview := false
		for _, act := range notif.Activities {
			if act.Type == pullrequest.ActivityTypeComment && act.Count == 1 {
				hasComment = true
			}
			if act.Type == pullrequest.ActivityTypeReview && act.Count == 1 {
				hasReview = true
			}
		}
		return hasComment && hasReview
	})).Return(nil)

	// Act - send two separate activity events (as the aggregate would)
	err := handler.Handle(context.Background(), &commentEvent)
	require.NoError(t, err)

	err = handler.Handle(context.Background(), &reviewEvent)
	require.NoError(t, err)

	// Wait for aggregator to flush
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleMultiplePRs_SeparateNotifications(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	event1 := pullrequest.NewNewPullRequestDetected(pr1)
	event2 := pullrequest.NewNewPullRequestDetected(pr2)

	// Mock expectations - should get 2 PRs in ONE batched notification call
	// Each adapter will then send one notification per PR
	mockNotificationPort.On("NotifyPullRequests", mock.AnythingOfType("[]*port.PRNotificationData")).
		Run(func(args mock.Arguments) {
			notifications := args.Get(0).([]*port.PRNotificationData)
			// Should have 2 notifications (one per PR)
			require.Len(t, notifications, 2)
			// Both should be new PRs
			require.True(t, notifications[0].IsNew)
			require.True(t, notifications[1].IsNew)
		}).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event1)
	require.NoError(t, err)

	err = handler.Handle(context.Background(), &event2)
	require.NoError(t, err)

	// Wait for aggregator to flush
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
	mockNotificationPort.AssertNumberOfCalls(t, "NotifyPullRequests", 1) // One batched call with 2 PRs
}

func TestNotificationHandler_HandleUnknownEvent_Ignored(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	// Create a mock event type (not a real event, just for testing)
	type UnknownEvent struct {
		pullrequest.Event
	}
	unknownEvent := &UnknownEvent{}

	// Act
	err := handler.Handle(context.Background(), unknownEvent)

	// Assert
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// No notification should be sent for unknown event types
	mockNotificationPort.AssertNotCalled(t, "NotifyPullRequests")
}

func TestNotificationHandler_HandlePRMerged_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewMerged(pr)

	// Mock expectations — merged events now send notifications
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		if len(notifications) != 1 {
			return false
		}
		notif := notifications[0]
		return notif.PullRequest == pr &&
			len(notif.StatusChanges) == 1 &&
			notif.StatusChanges[0].EventType == pullrequest.StatusChangeMerged
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)
	require.NoError(t, err)

	// Wait for aggregator to flush
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandlePRClosed_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewClosed(pr)

	// Mock expectations — closed events now send notifications
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		if len(notifications) != 1 {
			return false
		}
		notif := notifications[0]
		return notif.PullRequest == pr &&
			len(notif.StatusChanges) == 1 &&
			notif.StatusChanges[0].EventType == pullrequest.StatusChangeClosed
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)
	require.NoError(t, err)

	// Wait for aggregator to flush
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandleStatusChanged_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewStatusChanged(pr, pullrequest.StatusOpen, pullrequest.StatusMerged)

	// Act
	err := handler.Handle(context.Background(), &event)

	// Assert
	require.NoError(t, err)
	// Status changes are handled by specific events, so no notification expected
}

func TestNotificationHandler_ImmediateFlush_OnStop(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	// Mock expectations
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		return len(notifications) == 1 && notifications[0].IsNew
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)
	require.NoError(t, err)

	// Stop immediately - should flush pending notifications
	handler.Stop()

	// Assert - no need to wait, Stop() flushes immediately
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandlePipelineStatusChanged_Success(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewPipelineStatusChanged(pr, pullrequest.PipelineStatusUnknown, pullrequest.PipelineStatusFailed)

	// Mock expectations
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		if len(notifications) != 1 {
			return false
		}
		notif := notifications[0]
		return notif.PullRequest == pr &&
			notif.PipelineChange != nil &&
			notif.PipelineChange.OldStatus == pullrequest.PipelineStatusUnknown &&
			notif.PipelineChange.NewStatus == pullrequest.PipelineStatusFailed
	})).Return(nil)

	// Act
	err := handler.Handle(context.Background(), &event)
	require.NoError(t, err)

	// Wait for aggregator to flush
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
}

func TestNotificationHandler_HandlePipelineStatusChanged_GroupedWithActivity(t *testing.T) {
	// Arrange
	mockNotificationPort := mocks.NewNotificationPort(t)
	handler := events.NewNotificationEventHandler(mockNotificationPort, "")
	defer handler.Stop()

	pr := testutil.NewTestPullRequest(1)

	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)
	activityEvent := pullrequest.NewActivityDetected(pr, activity)
	pipelineEvent := pullrequest.NewPipelineStatusChanged(pr, pullrequest.PipelineStatusRunning, pullrequest.PipelineStatusSuccess)

	// Expect a single grouped notification with both activity and pipeline change
	mockNotificationPort.On("NotifyPullRequests", mock.MatchedBy(func(notifications []*port.PRNotificationData) bool {
		if len(notifications) != 1 {
			return false
		}
		notif := notifications[0]
		return notif.PullRequest == pr &&
			len(notif.Activities) == 1 &&
			notif.PipelineChange != nil &&
			notif.PipelineChange.NewStatus == pullrequest.PipelineStatusSuccess
	})).Return(nil)

	// Act
	_ = handler.Handle(context.Background(), &activityEvent)
	_ = handler.Handle(context.Background(), &pipelineEvent)

	// Wait for aggregator to flush
	time.Sleep(2200 * time.Millisecond)

	// Assert
	mockNotificationPort.AssertExpectations(t)
}
