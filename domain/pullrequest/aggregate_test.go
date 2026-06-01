package pullrequest_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPullRequest_ValidData(t *testing.T) {
	// Arrange
	url := "https://github.com/owner/repo/pull/123"
	number := 123
	title := "Test PR"
	repo := testutil.NewTestRepository("owner/repo")
	author := testutil.NewTestAuthor("testuser")
	createdAt := time.Now().Add(-1 * time.Hour)
	isDraft := false

	// Act
	pr, err := pullrequest.NewPullRequest(url, number, title, repo, author, createdAt, isDraft)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, pr)
	assert.Equal(t, url, pr.URL())
	assert.Equal(t, number, pr.Number())
	assert.Equal(t, title, pr.Title())
	assert.Equal(t, "owner/repo", pr.RepositoryName())
	assert.Equal(t, "testuser", pr.AuthorLogin())
	assert.True(t, pr.CreatedAt().Equal(createdAt))
	assert.False(t, pr.IsDraft())
	assert.True(t, pr.IsOpen())
	assert.False(t, pr.Seen())
}

func TestNewPullRequest_EmptyTitle_ReturnsError(t *testing.T) {
	// Arrange
	url := "https://github.com/owner/repo/pull/123"
	repo := testutil.NewTestRepository("owner/repo")
	author := testutil.NewTestAuthor("testuser")

	// Act
	pr, err := pullrequest.NewPullRequest(url, 123, "", repo, author, time.Now(), false)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, pr)
}

func TestNewPullRequest_ZeroCreatedAt_UsesCurrentTime(t *testing.T) {
	// Arrange
	before := time.Now()
	url := "https://github.com/owner/repo/pull/123"
	repo := testutil.NewTestRepository("owner/repo")
	author := testutil.NewTestAuthor("testuser")

	// Act
	pr, err := pullrequest.NewPullRequest(url, 123, "Test", repo, author, time.Time{}, false)
	after := time.Now()

	// Assert
	require.NoError(t, err)
	assert.True(t, pr.CreatedAt().After(before) || pr.CreatedAt().Equal(before))
	assert.True(t, pr.CreatedAt().Before(after) || pr.CreatedAt().Equal(after))
}

func TestPullRequest_AddActivity_UpdatesLastActivityTime(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	activityTime := time.Now().Add(1 * time.Hour)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		activityTime,
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)

	// Act
	pr.AddActivity(activity)

	// Assert
	assert.Len(t, pr.Activities(), 1)
	assert.True(t, pr.LastActivityAt().Equal(activityTime))
}

func TestPullRequest_AddActivity_MultipleActivities_UpdatesToLatest(t *testing.T) {
	// Arrange
	now := time.Now()
	// Create PR in the past (3 hours ago) so activities are more recent
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-3*time.Hour)))

	time1 := now.Add(-2 * time.Hour)
	time2 := now.Add(-1 * time.Hour)
	time3 := now.Add(-30 * time.Minute)

	activity1 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, time1)
	activity2 := testutil.NewTestActivity(pullrequest.ActivityTypeReview, time2)
	activity3 := testutil.NewTestActivity(pullrequest.ActivityTypeCommit, time3)

	// Act
	pr.AddActivity(activity1)
	pr.AddActivity(activity2)
	pr.AddActivity(activity3)

	// Assert
	assert.Len(t, pr.Activities(), 3)
	assert.True(t, pr.LastActivityAt().Equal(time3), "LastActivityAt should match the most recent activity time")
}

func TestPullRequest_AddActivity_NilActivity_DoesNothing(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	initialLastActivity := pr.LastActivityAt()

	// Act
	pr.AddActivity(nil)

	// Assert
	assert.Empty(t, pr.Activities())
	assert.True(t, pr.LastActivityAt().Equal(initialLastActivity))
}

func TestPullRequest_AddActivities_Multiple(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()

	activities := []*pullrequest.Activity{
		testutil.NewTestActivity(pullrequest.ActivityTypeComment, now.Add(-1*time.Hour)),
		testutil.NewTestActivity(pullrequest.ActivityTypeReview, now.Add(-30*time.Minute)),
		testutil.NewTestActivity(pullrequest.ActivityTypeCommit, now.Add(-15*time.Minute)),
	}

	// Act
	pr.AddActivities(activities)

	// Assert
	assert.Len(t, pr.Activities(), 3)
}

func TestPullRequest_ActivitiesSince_ReturnsCorrectActivities(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()
	since := now.Add(-1 * time.Hour)

	oldActivity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now.Add(-2*time.Hour))
	recentActivity1 := testutil.NewTestActivity(pullrequest.ActivityTypeReview, now.Add(-30*time.Minute))
	recentActivity2 := testutil.NewTestActivity(pullrequest.ActivityTypeCommit, now.Add(-15*time.Minute))

	pr.AddActivities([]*pullrequest.Activity{oldActivity, recentActivity1, recentActivity2})

	// Act
	recent := pr.ActivitiesSince(since)

	// Assert
	assert.Len(t, recent, 2, "Should return only activities after 'since' time")
}

func TestPullRequest_HasActivitiesSince_True(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()
	since := now.Add(-1 * time.Hour)

	activity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now.Add(-30*time.Minute))
	pr.AddActivity(activity)

	// Act & Assert
	assert.True(t, pr.HasActivitiesSince(since))
}

func TestPullRequest_HasActivitiesSince_False(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()
	since := now.Add(-1 * time.Hour)

	activity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now.Add(-2*time.Hour))
	pr.AddActivity(activity)

	// Act & Assert
	assert.False(t, pr.HasActivitiesSince(since))
}

func TestPullRequest_HasActivitiesSince_EmptyActivities(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	since := time.Now().Add(-1 * time.Hour)

	// Act & Assert
	assert.False(t, pr.HasActivitiesSince(since))
}

func TestPullRequest_ClearActivities(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()

	activities := []*pullrequest.Activity{
		testutil.NewTestActivity(pullrequest.ActivityTypeComment, now.Add(-1*time.Hour)),
		testutil.NewTestActivity(pullrequest.ActivityTypeReview, now.Add(-30*time.Minute)),
	}
	pr.AddActivities(activities)

	// Act
	pr.ClearActivities()

	// Assert
	assert.Empty(t, pr.Activities())
	assert.True(t, pr.LastActivityAt().Equal(pr.CreatedAt()))
}

func TestPullRequest_Activities_ReturnsEncapsulatedCopy(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, time.Now())
	pr.AddActivity(activity)

	// Act
	activities := pr.Activities()
	activities[0] = nil // Try to modify the returned slice

	// Assert
	assert.NotNil(t, pr.Activities()[0], "Original activities should not be affected")
	assert.Len(t, pr.Activities(), 1)
}

func TestPullRequest_Close(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	assert.True(t, pr.IsOpen())

	// Act
	pr.Close()

	// Assert
	assert.False(t, pr.IsOpen())
	assert.Equal(t, pullrequest.StatusClosed, pr.Status())
}

func TestPullRequest_Merge(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	assert.True(t, pr.IsOpen())

	// Act
	pr.Merge()

	// Assert
	assert.False(t, pr.IsOpen())
	assert.Equal(t, pullrequest.StatusMerged, pr.Status())
}

func TestPullRequest_IsStale(t *testing.T) {
	// Arrange
	now := time.Now()
	oldPR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	recentPR := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-1*time.Hour)))

	threshold := 48 * time.Hour

	// Act & Assert — use fixed now via IsStaleAt for deterministic evaluation
	assert.True(t, oldPR.IsStaleAt(now, threshold))
	assert.False(t, recentPR.IsStaleAt(now, threshold))
}

func TestPullRequest_Age(t *testing.T) {
	// Arrange
	now := time.Now()
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-24*time.Hour)))

	// Act — use fixed now via AgeAt for a deterministic, exact assertion
	age := pr.AgeAt(now)

	// Assert
	assert.Equal(t, 24*time.Hour, age)
}

func TestPullRequest_Equals(t *testing.T) {
	// Arrange
	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr3 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	// Act & Assert
	assert.True(t, pr1.Equals(pr2))
	assert.False(t, pr1.Equals(pr3))
}

func TestPullRequest_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		isDraft        bool
		activityCount  int
		expectedDraft  bool
		expectedActLen int
	}{
		{
			name:           "draft PR with activities",
			isDraft:        true,
			activityCount:  3,
			expectedDraft:  true,
			expectedActLen: 3,
		},
		{
			name:           "regular PR without activities",
			isDraft:        false,
			activityCount:  0,
			expectedDraft:  false,
			expectedActLen: 0,
		},
		{
			name:           "regular PR with activities",
			isDraft:        false,
			activityCount:  5,
			expectedDraft:  false,
			expectedActLen: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			pr := testutil.NewTestPullRequest(1, testutil.WithDraft(tt.isDraft))

			for i := 0; i < tt.activityCount; i++ {
				activity := testutil.NewTestActivity(
					pullrequest.ActivityTypeComment,
					time.Now().Add(time.Duration(-i)*time.Minute),
				)
				pr.AddActivity(activity)
			}

			// Assert
			assert.Equal(t, tt.expectedDraft, pr.IsDraft())
			assert.Len(t, pr.Activities(), tt.expectedActLen)
		})
	}
}

func TestPullRequest_MarkAsNewlyDetected_RaisesEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	events := pr.MarkAsNewlyDetected()

	// Assert
	require.Len(t, events, 1)
	event, ok := events[0].(*pullrequest.NewPullRequestDetected)
	require.True(t, ok, "Expected NewPullRequestDetected event")
	assert.Equal(t, pr.Identifier(), event.PullRequestID)
	assert.Equal(t, pr.Repository(), event.Repository)
	assert.Equal(t, pr.Author(), event.Author)
	assert.Equal(t, pr, event.PullRequest)
}

func TestPullRequest_AddActivity_RaisesActivityDetectedEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, time.Now())

	// Act
	events := pr.AddActivity(activity)

	// Assert
	require.Len(t, events, 1)
	event, ok := events[0].(*pullrequest.ActivityDetected)
	require.True(t, ok, "Expected ActivityDetected event")
	assert.Equal(t, pr.Identifier(), event.PullRequestID)
	assert.Equal(t, pr.Repository(), event.Repository)
	assert.Equal(t, activity, event.Activity)
	assert.Equal(t, pr, event.PullRequest)
}

func TestPullRequest_AddActivity_Nil_NoEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	events := pr.AddActivity(nil)

	// Assert
	assert.Len(t, events, 0, "Should not raise event for nil activity")
}

func TestPullRequest_Close_RaisesClosedEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	events := pr.Close()

	// Assert
	require.Len(t, events, 1)
	event, ok := events[0].(*pullrequest.Closed)
	require.True(t, ok, "Expected Closed event")
	assert.Equal(t, pr.Identifier(), event.PullRequestID)
	assert.Equal(t, pullrequest.StatusClosed, pr.Status())
}

func TestPullRequest_Merge_RaisesMergedEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	events := pr.Merge()

	// Assert
	require.Len(t, events, 1)
	event, ok := events[0].(*pullrequest.Merged)
	require.True(t, ok, "Expected Merged event")
	assert.Equal(t, pr.Identifier(), event.PullRequestID)
	assert.Equal(t, pullrequest.StatusMerged, pr.Status())
}

func TestPullRequest_CloseAlreadyClosed_NoEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	pr.Close() // first close — events discarded

	// Act
	events := pr.Close()

	// Assert
	assert.Len(t, events, 0, "Should not raise event when already closed")
}

func TestPullRequest_MergeAlreadyMerged_NoEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	pr.Merge() // first merge — events discarded

	// Act
	events := pr.Merge()

	// Assert
	assert.Len(t, events, 0, "Should not raise event when already merged")
}

func TestPullRequest_CloseIdempotent_SecondCallReturnsNil(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	events1 := pr.Close()
	events2 := pr.Close()

	// Assert
	assert.Len(t, events1, 1, "First close should return an event")
	assert.Len(t, events2, 0, "Second close should return nothing")
}

func TestPullRequest_MultipleEvents_CollectedInOrder(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, time.Now())

	// Act — each command returns its events; caller collects in order
	e1 := pr.AddActivity(activity)
	e2 := pr.MarkAsNewlyDetected()
	e3 := pr.Close()
	events := append(append(e1, e2...), e3...)

	// Assert
	require.Len(t, events, 3)
	_, ok1 := events[0].(*pullrequest.ActivityDetected)
	_, ok2 := events[1].(*pullrequest.NewPullRequestDetected)
	_, ok3 := events[2].(*pullrequest.Closed)
	assert.True(t, ok1, "First event should be ActivityDetected")
	assert.True(t, ok2, "Second event should be NewPullRequestDetected")
	assert.True(t, ok3, "Third event should be Closed")
}

func TestPullRequest_RecordHeadCommitUpdate_FirstTime_InitializesWithoutActivity(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	events := pr.RecordHeadCommitUpdate("abc123")

	// Assert
	assert.Equal(t, "abc123", pr.HeadCommitSHA())
	assert.Empty(t, pr.Activities(), "First time should not create push activity")
	assert.Empty(t, events, "First time should not raise events")
}

func TestPullRequest_RecordHeadCommitUpdate_SameSHA_NoActivity(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	pr.SetHeadCommitSHA("abc123")

	// Act
	events := pr.RecordHeadCommitUpdate("abc123")

	// Assert
	assert.Empty(t, pr.Activities(), "Same SHA should not create push activity")
	assert.Empty(t, events, "Same SHA should not raise events")
}

func TestPullRequest_RecordHeadCommitUpdate_Changed_CreatesPushActivity(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1, testutil.WithAuthor("alice"))
	pr.SetHeadCommitSHA("abc123")

	// Act
	pr.RecordHeadCommitUpdate("def456")

	// Assert
	assert.Equal(t, "def456", pr.HeadCommitSHA())
	require.Len(t, pr.Activities(), 1)
	assert.Equal(t, pullrequest.ActivityTypePush, pr.Activities()[0].Type())
	assert.Equal(t, "def456", pr.Activities()[0].Body())
	assert.Equal(t, "alice", pr.Activities()[0].Author().Login())
}

func TestPullRequest_RecordHeadCommitUpdate_SelfPush_CreatesActivity(t *testing.T) {
	// Arrange - PR author is the authenticated user
	// The aggregate records all domain facts. Notification filtering is done downstream.
	pr := testutil.NewTestPullRequest(1, testutil.WithAuthor("testuser"))
	pr.SetHeadCommitSHA("abc123")

	// Act
	pr.RecordHeadCommitUpdate("def456")

	// Assert - activity IS created (it's a domain fact)
	assert.Equal(t, "def456", pr.HeadCommitSHA())
	require.Len(t, pr.Activities(), 1)
	assert.Equal(t, pullrequest.ActivityTypePush, pr.Activities()[0].Type())
	assert.Equal(t, "testuser", pr.Activities()[0].Author().Login())
}

func TestPullRequest_RecordHeadCommitUpdate_AlwaysCreatesPushActivity(t *testing.T) {
	// Arrange - the aggregate always records pushes as domain facts
	pr := testutil.NewTestPullRequest(1, testutil.WithAuthor("alice"))
	pr.SetHeadCommitSHA("abc123")

	// Act
	pr.RecordHeadCommitUpdate("def456")

	// Assert
	require.Len(t, pr.Activities(), 1)
	assert.Equal(t, pullrequest.ActivityTypePush, pr.Activities()[0].Type())
}

// --- Review tracking tests ---

func TestPullRequest_AddReview_RaisesReviewStateChangedEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	reviewer := testutil.NewTestAuthor("joe")
	review := pullrequest.NewReview(reviewer, pullrequest.ReviewStateApproved, time.Now())

	// Act
	events := pr.AddReview(review)

	// Assert
	require.Len(t, events, 1)

	reviewEvent, ok := events[0].(*pullrequest.ReviewStateChanged)
	require.True(t, ok, "event should be ReviewStateChanged")
	assert.Equal(t, "joe", reviewEvent.Reviewer.Login())
	assert.Equal(t, pullrequest.ReviewStateApproved, reviewEvent.State)
}

func TestPullRequest_AddReview_SameState_NoEvent(t *testing.T) {
	// Arrange - set initial review state
	pr := testutil.NewTestPullRequest(1)
	initialReviews := map[string]*pullrequest.Review{
		"joe": pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, time.Now()),
	}
	pr.SetReviews(initialReviews)

	// Act - add the same review state again
	review := pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, time.Now())
	events := pr.AddReview(review)

	// Assert - no events should be raised
	assert.Empty(t, events, "No event should be raised for same review state")
}

func TestPullRequest_AddReview_StateChange_RaisesEvent(t *testing.T) {
	// Arrange - set initial review as changes_requested
	pr := testutil.NewTestPullRequest(1)
	initialReviews := map[string]*pullrequest.Review{
		"joe": pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}
	pr.SetReviews(initialReviews)

	// Act - reviewer now approves
	review := pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, time.Now())
	events := pr.AddReview(review)

	// Assert - event should be raised for the state change
	require.Len(t, events, 1)

	reviewEvent, ok := events[0].(*pullrequest.ReviewStateChanged)
	require.True(t, ok)
	assert.Equal(t, "joe", reviewEvent.Reviewer.Login())
	assert.Equal(t, pullrequest.ReviewStateApproved, reviewEvent.State)
}

func TestPullRequest_AddReview_NilReview_DoesNothing(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	pr.AddReview(nil)

	// Assert
	assert.Empty(t, pr.Reviews())
	assert.Empty(t, pr.AddReview(nil))
}

func TestPullRequest_AddReview_MultipleReviewers(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act - multiple reviewers leave different reviews
	e1 := pr.AddReview(pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, time.Now()))
	e2 := pr.AddReview(pullrequest.NewReview(testutil.NewTestAuthor("alice"), pullrequest.ReviewStateChangesRequested, time.Now()))
	e3 := pr.AddReview(pullrequest.NewReview(testutil.NewTestAuthor("bob"), pullrequest.ReviewStateCommented, time.Now()))

	// Assert
	reviews := pr.Reviews()
	assert.Len(t, reviews, 3)
	assert.Equal(t, pullrequest.ReviewStateApproved, reviews["joe"].State())
	assert.Equal(t, pullrequest.ReviewStateChangesRequested, reviews["alice"].State())
	assert.Equal(t, pullrequest.ReviewStateCommented, reviews["bob"].State())

	// 3 events should have been raised (one per new review)
	events := append(append(e1, e2...), e3...)
	assert.Len(t, events, 3)
}

func TestPullRequest_SetReviews_NoEvents(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	reviews := map[string]*pullrequest.Review{
		"joe":   pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, time.Now()),
		"alice": pullrequest.NewReview(testutil.NewTestAuthor("alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}

	// Act
	pr.SetReviews(reviews)

	// Assert - SetReviews is a pure state setter; it never raises domain events.
	assert.Len(t, pr.Reviews(), 2)
}

func TestPullRequest_ReviewSummary(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	reviews := map[string]*pullrequest.Review{
		"Joe":   pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
		"Alice": pullrequest.NewReview(testutil.NewTestAuthor("Alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}
	pr.SetReviews(reviews)

	// Act
	summary := pr.ReviewSummary()

	// Assert
	assert.False(t, summary.IsEmpty())
	assert.Len(t, summary.ReviewersByState(pullrequest.ReviewStateApproved), 1)
	assert.Len(t, summary.ReviewersByState(pullrequest.ReviewStateChangesRequested), 1)
}

func TestPullRequest_ReviewSummary_Empty(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	summary := pr.ReviewSummary()

	// Assert
	assert.True(t, summary.IsEmpty())
	assert.Empty(t, summary.ReviewersByState(pullrequest.ReviewStateApproved))
}

func TestPullRequest_Reviews_ReturnsCopy(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	pr.AddReview(pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, time.Now()))

	// Act
	reviews := pr.Reviews()
	delete(reviews, "joe")

	// Assert - original should be unchanged
	assert.Len(t, pr.Reviews(), 1)
}

// Pipeline status tests

func TestPullRequest_PipelineStatus_DefaultIsUnknown(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Assert
	assert.Equal(t, pullrequest.PipelineStatusUnknown, pr.PipelineStatus())
}

func TestPullRequest_UpdatePipelineStatus_ChangesStatus(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	pr.UpdatePipelineStatus(pullrequest.PipelineStatusRunning)

	// Assert
	assert.Equal(t, pullrequest.PipelineStatusRunning, pr.PipelineStatus())
}

func TestPullRequest_UpdatePipelineStatus_RaisesEvent_WhenStatusChanges(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	events := pr.UpdatePipelineStatus(pullrequest.PipelineStatusRunning)

	// Assert
	require.Len(t, events, 1)

	pipelineEvent, ok := events[0].(*pullrequest.PipelineStatusChanged)
	require.True(t, ok, "expected PipelineStatusChanged event")
	assert.Equal(t, pullrequest.PipelineStatusUnknown, pipelineEvent.OldStatus)
	assert.Equal(t, pullrequest.PipelineStatusRunning, pipelineEvent.NewStatus)
}

func TestPullRequest_UpdatePipelineStatus_NoEvent_WhenStatusUnchanged(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	pr.UpdatePipelineStatus(pullrequest.PipelineStatusRunning) // first change — events discarded

	// Act - update with same status
	events := pr.UpdatePipelineStatus(pullrequest.PipelineStatusRunning)

	// Assert
	assert.Empty(t, events)
}

func TestPullRequest_UpdatePipelineStatus_MultipleTransitions(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act: unknown -> running -> success
	e1 := pr.UpdatePipelineStatus(pullrequest.PipelineStatusRunning)
	e2 := pr.UpdatePipelineStatus(pullrequest.PipelineStatusSuccess)
	events := append(e1, e2...)

	// Assert
	assert.Equal(t, pullrequest.PipelineStatusSuccess, pr.PipelineStatus())
	require.Len(t, events, 2)

	ev1 := events[0].(*pullrequest.PipelineStatusChanged)
	assert.Equal(t, pullrequest.PipelineStatusUnknown, ev1.OldStatus)
	assert.Equal(t, pullrequest.PipelineStatusRunning, ev1.NewStatus)

	ev2 := events[1].(*pullrequest.PipelineStatusChanged)
	assert.Equal(t, pullrequest.PipelineStatusRunning, ev2.OldStatus)
	assert.Equal(t, pullrequest.PipelineStatusSuccess, ev2.NewStatus)
}

func TestPullRequest_UpdatePipelineStatus_EventCarriesFullPR(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(42)

	// Act
	events := pr.UpdatePipelineStatus(pullrequest.PipelineStatusFailed)

	// Assert
	require.Len(t, events, 1)

	pipelineEvent := events[0].(*pullrequest.PipelineStatusChanged)
	assert.Equal(t, pr, pipelineEvent.PullRequest)
	assert.Equal(t, pr.Identifier(), pipelineEvent.PullRequestID)
	assert.Equal(t, pr.Repository(), pipelineEvent.Repository)
	assert.False(t, pipelineEvent.OccurredAt().IsZero())
}

// ── Seen state ────────────────────────────────────────────────────────────────

func TestPullRequest_Seen_DefaultFalse(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)

	assert.False(t, pr.Seen())
	assert.False(t, pr.IsSeen())
}

func TestPullRequest_MarkAsSeen(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)

	pr.MarkAsSeen()

	assert.True(t, pr.Seen())
	assert.True(t, pr.IsSeen())
}

func TestPullRequest_MarkAsUnseen(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	pr.MarkAsSeen()

	pr.MarkAsUnseen()

	assert.False(t, pr.Seen())
	assert.False(t, pr.IsSeen())
}

func TestPullRequest_SeenStateTransitions(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)

	assert.False(t, pr.Seen())

	pr.MarkAsSeen()
	assert.True(t, pr.Seen())

	pr.MarkAsUnseen()
	assert.False(t, pr.Seen())

	pr.MarkAsSeen()
	assert.True(t, pr.Seen())
}

func TestReconstitutePR_SeenTrue(t *testing.T) {
	identifier, err := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/1", 1)
	require.NoError(t, err)

	pr := pullrequest.ReconstitutePR(
		identifier,
		"Test PR",
		testutil.NewTestRepository("owner/repo"),
		testutil.NewTestAuthor("alice"),
		pullrequest.StatusOpen,
		time.Now(),
		false,
		nil,
		time.Time{},
		"",
		nil,
		pullrequest.PipelineStatusUnknown,
		true,
	)

	assert.True(t, pr.Seen())
}

func TestReconstitutePR_SeenFalse(t *testing.T) {
	identifier, err := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/2", 2)
	require.NoError(t, err)

	pr := pullrequest.ReconstitutePR(
		identifier,
		"Test PR",
		testutil.NewTestRepository("owner/repo"),
		testutil.NewTestAuthor("alice"),
		pullrequest.StatusOpen,
		time.Now(),
		false,
		nil,
		time.Time{},
		"",
		nil,
		pullrequest.PipelineStatusUnknown,
		false,
	)

	assert.False(t, pr.Seen())
}

func TestReconstitutePR_LastActivityAt_FallsBackToCreatedAt(t *testing.T) {
	// Regression: ReconstitutePR used to leave lastActivityAt as zero value.
	// It must fall back to createdAt when no activities are provided.
	createdAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	identifier, err := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/3", 3)
	require.NoError(t, err)

	pr := pullrequest.ReconstitutePR(
		identifier,
		"Test PR",
		testutil.NewTestRepository("owner/repo"),
		testutil.NewTestAuthor("alice"),
		pullrequest.StatusOpen,
		createdAt,
		false,
		nil,
		time.Time{},
		"",
		nil,
		pullrequest.PipelineStatusUnknown,
		false,
	)

	assert.True(t, pr.LastActivityAt().Equal(createdAt),
		"LastActivityAt should fall back to createdAt when no activities are present, got %v", pr.LastActivityAt())
}

func TestReconstitutePR_LastActivityAt_DerivedFromActivities(t *testing.T) {
	// When activities are passed, lastActivityAt should be the max activity time.
	createdAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	activityTime := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)
	identifier, err := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/4", 4)
	require.NoError(t, err)

	author := testutil.NewTestAuthor("alice")
	activity := pullrequest.NewActivity(identifier, pullrequest.ActivityTypeComment, author, activityTime, "hello")

	pr := pullrequest.ReconstitutePR(
		identifier,
		"Test PR",
		testutil.NewTestRepository("owner/repo"),
		testutil.NewTestAuthor("alice"),
		pullrequest.StatusOpen,
		createdAt,
		false,
		[]*pullrequest.Activity{activity},
		time.Time{},
		"",
		nil,
		pullrequest.PipelineStatusUnknown,
		false,
	)

	assert.True(t, pr.LastActivityAt().Equal(activityTime),
		"LastActivityAt should reflect the most recent activity, got %v", pr.LastActivityAt())
}

func TestReconstitutePR_DefensiveCopies(t *testing.T) {
	// Caller-owned maps/slices must not alias aggregate internals.
	createdAt := time.Now()
	identifier, err := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/5", 5)
	require.NoError(t, err)

	callerReviews := map[string]*pullrequest.Review{
		"alice": pullrequest.NewReview(testutil.NewTestAuthor("alice"), pullrequest.ReviewStateApproved, time.Now()),
	}

	pr := pullrequest.ReconstitutePR(
		identifier,
		"Test PR",
		testutil.NewTestRepository("owner/repo"),
		testutil.NewTestAuthor("alice"),
		pullrequest.StatusOpen,
		createdAt,
		false,
		nil,
		time.Time{},
		"",
		callerReviews,
		pullrequest.PipelineStatusUnknown,
		false,
	)

	// Mutate the caller's map after construction
	delete(callerReviews, "alice")

	// Aggregate must still have the review
	assert.Len(t, pr.Reviews(), 1, "aggregate reviews must not alias the caller's map")
}

func TestSetReviews_DefensiveCopy(t *testing.T) {
	// Caller-owned map must not alias aggregate internals after SetReviews.
	pr := testutil.NewTestPullRequest(1)
	callerReviews := map[string]*pullrequest.Review{
		"bob": pullrequest.NewReview(testutil.NewTestAuthor("bob"), pullrequest.ReviewStateApproved, time.Now()),
	}

	pr.SetReviews(callerReviews)
	delete(callerReviews, "bob")

	assert.Len(t, pr.Reviews(), 1, "aggregate reviews must not alias the caller's map")
}
