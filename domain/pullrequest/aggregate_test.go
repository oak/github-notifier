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

	// Act & Assert
	assert.True(t, oldPR.IsStale(threshold))
	assert.False(t, recentPR.IsStale(threshold))
}

func TestPullRequest_Age(t *testing.T) {
	// Arrange
	now := time.Now()
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-24*time.Hour)))

	// Act
	age := pr.Age()

	// Assert
	assert.True(t, age >= 23*time.Hour, "Age should be at least 23 hours")
	assert.True(t, age <= 25*time.Hour, "Age should be at most 25 hours")
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
	pr.MarkAsNewlyDetected()
	events := pr.CollectEvents()

	// Assert
	require.Len(t, events, 1)
	event, ok := events[0].(*pullrequest.NewPullRequestDetected)
	require.True(t, ok, "Expected NewPullRequestDetected event")
	assert.Equal(t, pr.Identifier(), event.PullRequestID)
	assert.Equal(t, pr.Repository(), event.Repository)
	assert.Equal(t, pr.Author(), event.Author)
	assert.Equal(t, pr, event.PullRequest)
}

func TestPullRequest_RecordNewActivity_RaisesEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, time.Now())
	pr.AddActivity(activity)

	// Act
	pr.RecordNewActivity()
	events := pr.CollectEvents()

	// Assert
	require.Len(t, events, 1)
	event, ok := events[0].(*pullrequest.ActivityDetected)
	require.True(t, ok, "Expected ActivityDetected event")
	assert.Equal(t, pr.Identifier(), event.PullRequestID)
	assert.Equal(t, pr.Repository(), event.Repository)
	assert.Len(t, event.Activities, 1)
	assert.Equal(t, pr, event.PullRequest)
}

func TestPullRequest_RecordNewActivity_NoActivities_NoEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	pr.RecordNewActivity()
	events := pr.CollectEvents()

	// Assert
	assert.Len(t, events, 0, "Should not raise event when there are no activities")
}

func TestPullRequest_Close_RaisesStatusChangedEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	pr.Close()
	events := pr.CollectEvents()

	// Assert
	require.Len(t, events, 1)
	event, ok := events[0].(*pullrequest.StatusChanged)
	require.True(t, ok, "Expected StatusChanged event")
	assert.Equal(t, pr.Identifier(), event.PullRequestID)
	assert.Equal(t, pullrequest.StatusOpen, event.OldStatus)
	assert.Equal(t, pullrequest.StatusClosed, event.NewStatus)
}

func TestPullRequest_Merge_RaisesStatusChangedEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	// Act
	pr.Merge()
	events := pr.CollectEvents()

	// Assert
	require.Len(t, events, 1)
	event, ok := events[0].(*pullrequest.StatusChanged)
	require.True(t, ok, "Expected StatusChanged event")
	assert.Equal(t, pr.Identifier(), event.PullRequestID)
	assert.Equal(t, pullrequest.StatusOpen, event.OldStatus)
	assert.Equal(t, pullrequest.StatusMerged, event.NewStatus)
}

func TestPullRequest_CloseAlreadyClosed_NoEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	pr.Close()
	pr.CollectEvents() // Clear events

	// Act
	pr.Close()
	events := pr.CollectEvents()

	// Assert
	assert.Len(t, events, 0, "Should not raise event when already closed")
}

func TestPullRequest_MergeAlreadyMerged_NoEvent(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	pr.Merge()
	pr.CollectEvents() // Clear events

	// Act
	pr.Merge()
	events := pr.CollectEvents()

	// Assert
	assert.Len(t, events, 0, "Should not raise event when already merged")
}

func TestPullRequest_CollectEvents_ClearsEventList(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	pr.MarkAsNewlyDetected()

	// Act
	events1 := pr.CollectEvents()
	events2 := pr.CollectEvents()

	// Assert
	assert.Len(t, events1, 1, "First collection should have events")
	assert.Len(t, events2, 0, "Second collection should be empty")
}

func TestPullRequest_MultipleEvents_CollectedInOrder(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(pullrequest.ActivityTypeComment, time.Now())
	pr.AddActivity(activity)

	// Act
	pr.MarkAsNewlyDetected()
	pr.RecordNewActivity()
	pr.Close()
	events := pr.CollectEvents()

	// Assert
	require.Len(t, events, 3)
	_, ok1 := events[0].(*pullrequest.NewPullRequestDetected)
	_, ok2 := events[1].(*pullrequest.ActivityDetected)
	_, ok3 := events[2].(*pullrequest.StatusChanged)
	assert.True(t, ok1, "First event should be NewPullRequestDetected")
	assert.True(t, ok2, "Second event should be ActivityDetected")
	assert.True(t, ok3, "Third event should be StatusChanged")
}
