package pullrequest_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestClassifyPRs_AllNew(t *testing.T) {
	// Arrange
	since := time.Now().Add(-1 * time.Hour)

	// Create PRs without activities
	prs := testutil.CreateTestPRs(3, 0)

	// Act
	trulyNew, withActivity := pullrequest.ClassifyPRs(prs, since)

	// Assert
	assert.Len(t, trulyNew, 3, "All PRs should be classified as truly new")
	assert.Empty(t, withActivity, "No PRs should have activity")
}

func TestClassifyPRs_AllWithActivity(t *testing.T) {
	// Arrange
	since := time.Now().Add(-1 * time.Hour)

	// Create PRs with recent activities (30 minutes ago)
	prs := testutil.CreateTestPRsWithActivities(3, 2, 30*time.Minute)

	// Act
	trulyNew, withActivity := pullrequest.ClassifyPRs(prs, since)

	// Assert
	assert.Empty(t, trulyNew, "No PRs should be classified as truly new")
	assert.Len(t, withActivity, 3, "All PRs should have activity")
}

func TestClassifyPRs_Mixed(t *testing.T) {
	// Arrange
	since := time.Now().Add(-1 * time.Hour)

	// Create 2 PRs without activities
	newPRs := testutil.CreateTestPRs(2, 0)

	// Create 3 PRs with recent activities
	activePRs := testutil.CreateTestPRsWithActivities(3, 2, 30*time.Minute)

	allPRs := append(newPRs, activePRs...)

	// Act
	trulyNew, withActivity := pullrequest.ClassifyPRs(allPRs, since)

	// Assert
	assert.Len(t, trulyNew, 2, "Should have 2 truly new PRs")
	assert.Len(t, withActivity, 3, "Should have 3 PRs with activity")
}

func TestClassifyPRs_EmptyInput(t *testing.T) {
	// Arrange
	since := time.Now().Add(-1 * time.Hour)
	var prs []*pullrequest.PullRequest

	// Act
	trulyNew, withActivity := pullrequest.ClassifyPRs(prs, since)

	// Assert
	assert.Empty(t, trulyNew, "Should have no truly new PRs")
	assert.Empty(t, withActivity, "Should have no PRs with activity")
}

func TestClassifyPRs_OldActivities(t *testing.T) {
	// Arrange
	since := time.Now().Add(-1 * time.Hour)

	// Create PRs with old activities (2 hours ago, before "since" time)
	prs := testutil.CreateTestPRsWithActivities(3, 2, 2*time.Hour)

	// Act
	trulyNew, withActivity := pullrequest.ClassifyPRs(prs, since)

	// Assert
	assert.Len(t, trulyNew, 3, "PRs with only old activities should be classified as truly new")
	assert.Empty(t, withActivity, "No PRs should have recent activity")
}

func TestClassifyPRs_SinceBoundary(t *testing.T) {
	// Arrange
	now := time.Now()
	since := now.Add(-1 * time.Hour)

	// Create PR with activity exactly at "since" time
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		since,
		testutil.WithActivityPR(pr.URL(), pr.Number()),
	)
	pr.AddActivities([]*pullrequest.Activity{activity})

	// Act
	trulyNew, withActivity := pullrequest.ClassifyPRs([]*pullrequest.PullRequest{pr}, since)

	// Assert
	// Activity exactly at "since" time is not included (After, not AfterOrEqual)
	assert.Len(t, trulyNew, 1, "PR with activity exactly at boundary should be truly new (not After)")
	assert.Empty(t, withActivity, "PR should not have activity at exact boundary (After, not AfterOrEqual)")
}

func TestClassifyPRs_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		newPRCount          int
		activePRCount       int
		activityAge         time.Duration
		sinceAge            time.Duration
		expectedNewCount    int
		expectedActiveCount int
	}{
		{
			name:                "all new PRs",
			newPRCount:          5,
			activePRCount:       0,
			activityAge:         0,
			sinceAge:            1 * time.Hour,
			expectedNewCount:    5,
			expectedActiveCount: 0,
		},
		{
			name:                "all active PRs",
			newPRCount:          0,
			activePRCount:       5,
			activityAge:         30 * time.Minute,
			sinceAge:            1 * time.Hour,
			expectedNewCount:    0,
			expectedActiveCount: 5,
		},
		{
			name:                "mixed PRs",
			newPRCount:          3,
			activePRCount:       2,
			activityAge:         30 * time.Minute,
			sinceAge:            1 * time.Hour,
			expectedNewCount:    3,
			expectedActiveCount: 2,
		},
		{
			name:                "old activities should be ignored",
			newPRCount:          0,
			activePRCount:       5,
			activityAge:         2 * time.Hour,
			sinceAge:            1 * time.Hour,
			expectedNewCount:    5,
			expectedActiveCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			since := time.Now().Add(-tt.sinceAge)

			newPRs := testutil.CreateTestPRs(tt.newPRCount, 0)
			activePRs := testutil.CreateTestPRsWithActivities(tt.activePRCount, 2, tt.activityAge)
			allPRs := append(newPRs, activePRs...)

			// Act
			trulyNew, withActivity := pullrequest.ClassifyPRs(allPRs, since)

			// Assert
			assert.Len(t, trulyNew, tt.expectedNewCount, "Should have expected count of truly new PRs")
			assert.Len(t, withActivity, tt.expectedActiveCount, "Should have expected count of PRs with activity")
		})
	}
}
