package pullrequest_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestActivityCheckScheduler_DeterminePRsToCheck_AllRecent(t *testing.T) {
	// Arrange
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15) // 48h recent, 15min stale interval
	now := time.Now()

	// Create PRs that are 1 hour old (recent)
	prs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-1*time.Hour))),
		testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-2*time.Hour))),
		testutil.NewTestPullRequest(3, testutil.WithCreatedAt(now.Add(-3*time.Hour))),
	}

	// Act
	result := scheduler.DeterminePRsToCheck(prs)

	// Assert
	assert.Len(t, result.PRsToCheck, 3, "All recent PRs should be checked")
	assert.Equal(t, 3, result.RecentCount, "Should have 3 recent PRs")
	assert.Equal(t, 0, result.StaleCount, "Should have 0 stale PRs")
	assert.Equal(t, 0, result.SkippedCount, "Should have 0 skipped PRs")
}

func TestActivityCheckScheduler_DeterminePRsToCheck_AllStaleFirstCheck(t *testing.T) {
	// Arrange
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15) // 48h recent, 15min stale interval
	now := time.Now()

	// Create PRs that are 72 hours old (stale)
	prs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour))),
		testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-96*time.Hour))),
	}

	// Act
	result := scheduler.DeterminePRsToCheck(prs)

	// Assert
	assert.Len(t, result.PRsToCheck, 2, "All stale PRs should be checked on first check")
	assert.Equal(t, 0, result.RecentCount, "Should have 0 recent PRs")
	assert.Equal(t, 2, result.StaleCount, "Should have 2 stale PRs")
	assert.Equal(t, 0, result.SkippedCount, "Should have 0 skipped PRs")
}

func TestActivityCheckScheduler_DeterminePRsToCheck_StaleRecentlyChecked(t *testing.T) {
	// Arrange
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15) // 48h recent, 15min stale interval
	now := time.Now()

	// Create stale PR
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	prs := []*pullrequest.PullRequest{pr}

	// Mark as checked 5 minutes ago
	scheduler.MarkChecked(prs)

	// Wait is simulated by testing immediately (less than 15min interval)
	// Act
	result := scheduler.DeterminePRsToCheck(prs)

	// Assert
	assert.Empty(t, result.PRsToCheck, "Stale PR checked recently should be skipped")
	assert.Equal(t, 0, result.RecentCount, "Should have 0 recent PRs")
	assert.Equal(t, 0, result.StaleCount, "Should have 0 stale PRs due for check")
	assert.Equal(t, 1, result.SkippedCount, "Should have 1 skipped PR")
}

func TestActivityCheckScheduler_DeterminePRsToCheck_Mixed(t *testing.T) {
	// Arrange
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15) // 48h recent, 15min stale interval
	now := time.Now()

	// Create mixed PRs
	recentPR1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	recentPR2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-24*time.Hour)))
	stalePR1 := testutil.NewTestPullRequest(3, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	stalePR2 := testutil.NewTestPullRequest(4, testutil.WithCreatedAt(now.Add(-96*time.Hour)))

	prs := []*pullrequest.PullRequest{recentPR1, recentPR2, stalePR1, stalePR2}

	// Act
	result := scheduler.DeterminePRsToCheck(prs)

	// Assert
	assert.Len(t, result.PRsToCheck, 4, "All PRs should be checked (recent always + stale first time)")
	assert.Equal(t, 2, result.RecentCount, "Should have 2 recent PRs")
	assert.Equal(t, 2, result.StaleCount, "Should have 2 stale PRs")
	assert.Equal(t, 0, result.SkippedCount, "Should have 0 skipped PRs")
}

func TestActivityCheckScheduler_DeterminePRsToCheck_RecentThresholdBoundary(t *testing.T) {
	// Arrange
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)
	now := time.Now()

	// Create PR exactly at the threshold (48 hours)
	prAtBoundary := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-48*time.Hour)))
	prJustBefore := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-47*time.Hour-59*time.Minute)))
	prJustAfter := testutil.NewTestPullRequest(3, testutil.WithCreatedAt(now.Add(-48*time.Hour-1*time.Minute)))

	prs := []*pullrequest.PullRequest{prAtBoundary, prJustBefore, prJustAfter}

	// Act
	result := scheduler.DeterminePRsToCheck(prs)

	// Assert
	assert.Len(t, result.PRsToCheck, 3, "All PRs should be checked on first check")
	assert.Equal(t, 1, result.RecentCount, "PR just before threshold should be recent")
	assert.Equal(t, 2, result.StaleCount, "PRs at/after threshold should be stale")
}

func TestActivityCheckScheduler_MarkChecked_UpdatesLastCheckTime(t *testing.T) {
	// Arrange
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)
	now := time.Now()
	stalePR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	prs := []*pullrequest.PullRequest{stalePR}

	// Act - First check should include the PR
	result1 := scheduler.DeterminePRsToCheck(prs)
	assert.Len(t, result1.PRsToCheck, 1, "First check should include stale PR")

	// Mark as checked
	scheduler.MarkChecked(prs)

	// Immediately check again
	result2 := scheduler.DeterminePRsToCheck(prs)

	// Assert
	assert.Empty(t, result2.PRsToCheck, "PR should be skipped after marking as checked")
	assert.Equal(t, 1, result2.SkippedCount, "Should skip the recently checked PR")
}

func TestActivityCheckScheduler_EmptyInput(t *testing.T) {
	// Arrange
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)
	var prs []*pullrequest.PullRequest

	// Act
	result := scheduler.DeterminePRsToCheck(prs)

	// Assert
	assert.Empty(t, result.PRsToCheck, "Should return empty result for empty input")
	assert.Equal(t, 0, result.RecentCount, "Should have 0 recent PRs")
	assert.Equal(t, 0, result.StaleCount, "Should have 0 stale PRs")
	assert.Equal(t, 0, result.SkippedCount, "Should have 0 skipped PRs")
}

func TestActivityCheckScheduler_TableDriven(t *testing.T) {
	tests := []struct {
		name                  string
		recentThresholdHours  int
		staleCheckIntervalMin int
		prAges                []time.Duration
		markCheckedIndexes    []int // which PRs to mark as checked before second determination
		expectedFirstCheck    int
		expectedSecondCheck   int
		expectedRecentSecond  int
		expectedStaleSecond   int
		expectedSkippedSecond int
	}{
		{
			name:                  "all recent PRs always checked",
			recentThresholdHours:  48,
			staleCheckIntervalMin: 15,
			prAges:                []time.Duration{1 * time.Hour, 2 * time.Hour, 24 * time.Hour},
			markCheckedIndexes:    []int{0, 1, 2},
			expectedFirstCheck:    3,
			expectedSecondCheck:   3, // Recent PRs always checked
			expectedRecentSecond:  3,
			expectedStaleSecond:   0,
			expectedSkippedSecond: 0,
		},
		{
			name:                  "stale PRs checked then skipped",
			recentThresholdHours:  48,
			staleCheckIntervalMin: 15,
			prAges:                []time.Duration{72 * time.Hour, 96 * time.Hour},
			markCheckedIndexes:    []int{0, 1},
			expectedFirstCheck:    2,
			expectedSecondCheck:   0, // Stale PRs skipped after marking
			expectedRecentSecond:  0,
			expectedStaleSecond:   0,
			expectedSkippedSecond: 2,
		},
		{
			name:                  "mixed recent and stale",
			recentThresholdHours:  48,
			staleCheckIntervalMin: 15,
			prAges:                []time.Duration{1 * time.Hour, 72 * time.Hour, 96 * time.Hour},
			markCheckedIndexes:    []int{0, 1, 2},
			expectedFirstCheck:    3,
			expectedSecondCheck:   1, // Only recent PR checked
			expectedRecentSecond:  1,
			expectedStaleSecond:   0,
			expectedSkippedSecond: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			scheduler := pullrequest.NewActivityCheckScheduler(tt.recentThresholdHours, tt.staleCheckIntervalMin)
			now := time.Now()

			prs := make([]*pullrequest.PullRequest, len(tt.prAges))
			for i, age := range tt.prAges {
				prs[i] = testutil.NewTestPullRequest(i+1, testutil.WithCreatedAt(now.Add(-age)))
			}

			// Act - First determination
			result1 := scheduler.DeterminePRsToCheck(prs)
			assert.Len(t, result1.PRsToCheck, tt.expectedFirstCheck, "First check should match expected")

			// Mark specified PRs as checked
			toMark := make([]*pullrequest.PullRequest, len(tt.markCheckedIndexes))
			for i, idx := range tt.markCheckedIndexes {
				toMark[i] = prs[idx]
			}
			scheduler.MarkChecked(toMark)

			// Act - Second determination (immediately after)
			result2 := scheduler.DeterminePRsToCheck(prs)

			// Assert
			assert.Len(t, result2.PRsToCheck, tt.expectedSecondCheck, "Second check should match expected")
			assert.Equal(t, tt.expectedRecentSecond, result2.RecentCount, "Recent count should match")
			assert.Equal(t, tt.expectedStaleSecond, result2.StaleCount, "Stale count should match")
			assert.Equal(t, tt.expectedSkippedSecond, result2.SkippedCount, "Skipped count should match")
		})
	}
}
