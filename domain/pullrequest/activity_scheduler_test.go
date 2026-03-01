package pullrequest_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

// newCfg is a convenience constructor to keep test declarations concise.
func newCfg(recentHours, staleMinutes int) pullrequest.SchedulerConfig {
	return pullrequest.SchedulerConfig{
		RecentThreshold:    time.Duration(recentHours) * time.Hour,
		StaleCheckInterval: time.Duration(staleMinutes) * time.Minute,
	}
}

// ─── DetermineChecks ─────────────────────────────────────────────────────────

func TestDetermineChecks_AllRecent(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := newCfg(48, 15)
	prs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-1*time.Hour))),
		testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-2*time.Hour))),
		testutil.NewTestPullRequest(3, testutil.WithCreatedAt(now.Add(-3*time.Hour))),
	}

	// Act
	toCheck, recentN, staleN, skippedN := pullrequest.DetermineChecks(prs, nil, cfg, now)

	// Assert
	assert.Len(t, toCheck, 3, "all recent PRs should be checked")
	assert.Equal(t, 3, recentN)
	assert.Equal(t, 0, staleN)
	assert.Equal(t, 0, skippedN)
}

func TestDetermineChecks_AllStaleFirstCheck(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := newCfg(48, 15)

	// Create PRs that are 72 hours old (stale)
	prs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour))),
		testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-96*time.Hour))),
	}

	// Act — nil lastChecked means no prior check; stale PRs always due on first call
	toCheck, recentN, staleN, skippedN := pullrequest.DetermineChecks(prs, nil, cfg, now)

	// Assert
	assert.Len(t, toCheck, 2, "all stale PRs should be checked on first check")
	assert.Equal(t, 0, recentN)
	assert.Equal(t, 2, staleN)
	assert.Equal(t, 0, skippedN)
}

func TestDetermineChecks_StaleRecentlyChecked(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := newCfg(48, 15)
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))

	// Mark as checked at the same instant — interval has not elapsed
	lastChecked := map[string]time.Time{pr.URL(): now}

	// Act
	toCheck, recentN, staleN, skippedN := pullrequest.DetermineChecks(
		[]*pullrequest.PullRequest{pr}, lastChecked, cfg, now,
	)

	// Assert
	assert.Empty(t, toCheck, "stale PR checked recently should be skipped")
	assert.Equal(t, 0, recentN)
	assert.Equal(t, 0, staleN)
	assert.Equal(t, 1, skippedN)
}

func TestDetermineChecks_Mixed(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := newCfg(48, 15)
	prs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-1*time.Hour))),
		testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-24*time.Hour))),
		testutil.NewTestPullRequest(3, testutil.WithCreatedAt(now.Add(-72*time.Hour))),
		testutil.NewTestPullRequest(4, testutil.WithCreatedAt(now.Add(-96*time.Hour))),
	}

	// Act
	toCheck, recentN, staleN, skippedN := pullrequest.DetermineChecks(prs, nil, cfg, now)

	// Assert
	assert.Len(t, toCheck, 4, "all PRs checked (recent always + stale first time)")
	assert.Equal(t, 2, recentN)
	assert.Equal(t, 2, staleN)
	assert.Equal(t, 0, skippedN)
}

func TestDetermineChecks_RecentThresholdBoundary(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := newCfg(48, 15)

	prAtBoundary := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-48*time.Hour)))
	prJustBefore := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-47*time.Hour-59*time.Minute)))
	prJustAfter := testutil.NewTestPullRequest(3, testutil.WithCreatedAt(now.Add(-48*time.Hour-1*time.Minute)))

	// Act
	toCheck, recentN, staleN, _ := pullrequest.DetermineChecks(
		[]*pullrequest.PullRequest{prAtBoundary, prJustBefore, prJustAfter},
		nil, cfg, now,
	)

	// Assert
	assert.Len(t, toCheck, 3, "all PRs checked on first check")
	assert.Equal(t, 1, recentN, "PR just before threshold should be recent")
	assert.Equal(t, 2, staleN, "PRs at/after threshold should be stale")
}

func TestDetermineChecks_EmptyInput(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := newCfg(48, 15)

	// Act
	toCheck, recentN, staleN, skippedN := pullrequest.DetermineChecks(nil, nil, cfg, now)

	// Assert
	assert.Empty(t, toCheck)
	assert.Equal(t, 0, recentN)
	assert.Equal(t, 0, staleN)
	assert.Equal(t, 0, skippedN)
}

func TestDetermineChecks_TableDriven(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		recentHours      int
		staleMinutes     int
		prAges           []time.Duration
		lastCheckedIdxs  []int // indexes into prs slice pre-populated as "checked at now"
		expectedTotal    int
		expectedRecentN  int
		expectedStaleN   int
		expectedSkippedN int
	}{
		{
			name:            "all recent PRs always checked",
			recentHours:     48,
			staleMinutes:    15,
			prAges:          []time.Duration{1 * time.Hour, 2 * time.Hour, 24 * time.Hour},
			lastCheckedIdxs: []int{0, 1, 2},
			expectedTotal:   3,
			expectedRecentN: 3,
		},
		{
			name:             "stale PRs skipped when interval not elapsed",
			recentHours:      48,
			staleMinutes:     15,
			prAges:           []time.Duration{72 * time.Hour, 96 * time.Hour},
			lastCheckedIdxs:  []int{0, 1},
			expectedTotal:    0,
			expectedSkippedN: 2,
		},
		{
			name:             "mixed: recent always, stale skipped after check",
			recentHours:      48,
			staleMinutes:     15,
			prAges:           []time.Duration{1 * time.Hour, 72 * time.Hour, 96 * time.Hour},
			lastCheckedIdxs:  []int{1, 2},
			expectedTotal:    1,
			expectedRecentN:  1,
			expectedSkippedN: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := newCfg(tt.recentHours, tt.staleMinutes)
			prs := make([]*pullrequest.PullRequest, len(tt.prAges))
			for i, age := range tt.prAges {
				prs[i] = testutil.NewTestPullRequest(i+1, testutil.WithCreatedAt(now.Add(-age)))
			}

			lastChecked := make(map[string]time.Time, len(tt.lastCheckedIdxs))
			for _, idx := range tt.lastCheckedIdxs {
				lastChecked[prs[idx].URL()] = now
			}

			// Act
			toCheck, recentN, staleN, skippedN := pullrequest.DetermineChecks(prs, lastChecked, cfg, now)

			// Assert
			assert.Len(t, toCheck, tt.expectedTotal)
			assert.Equal(t, tt.expectedRecentN, recentN)
			assert.Equal(t, tt.expectedStaleN, staleN)
			assert.Equal(t, tt.expectedSkippedN, skippedN)
		})
	}
}

// ─── RecordChecked ────────────────────────────────────────────────────────────

func TestRecordChecked_AddsEntries(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))

	// Act
	result := pullrequest.RecordChecked(nil, []*pullrequest.PullRequest{pr}, now)

	// Assert
	assert.Len(t, result, 1)
	assert.Equal(t, now, result[pr.URL()])
}

func TestRecordChecked_DoesNotMutateInput(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-96*time.Hour)))

	original := map[string]time.Time{pr1.URL(): now.Add(-1 * time.Hour)}

	// Act
	result := pullrequest.RecordChecked(original, []*pullrequest.PullRequest{pr2}, now)

	// Assert
	assert.Len(t, original, 1, "input map must not be mutated")
	assert.Len(t, result, 2)
	assert.Equal(t, now.Add(-1*time.Hour), result[pr1.URL()], "existing entry must be preserved")
	assert.Equal(t, now, result[pr2.URL()], "new entry must be added")
}

func TestRecordChecked_OverwritesExistingEntry(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	old := now.Add(-1 * time.Hour)

	original := map[string]time.Time{pr.URL(): old}

	// Act
	result := pullrequest.RecordChecked(original, []*pullrequest.PullRequest{pr}, now)

	// Assert
	assert.Equal(t, now, result[pr.URL()], "timestamp must be updated to now")
}

// ─── ActivityCheckScheduler (thin wrapper) ────────────────────────────────────

func TestActivityCheckScheduler_DelegatesCorrectly(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)
	stalePR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	prs := []*pullrequest.PullRequest{stalePR}

	// Act — first check includes the stale PR (never seen before)
	result1 := scheduler.DeterminePRsToCheckAt(now, prs)
	assert.Len(t, result1.PRsToCheck, 1, "stale PR should be checked first time")

	// Mark checked, then verify it is skipped at the same instant
	scheduler.MarkCheckedAt(now, prs)
	result2 := scheduler.DeterminePRsToCheckAt(now, prs)

	// Assert
	assert.Empty(t, result2.PRsToCheck, "stale PR should be skipped after marking")
	assert.Equal(t, 1, result2.SkippedCount)
}

func TestActivityCheckScheduler_SeedLastChecked(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)
	stalePR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))

	// Seed the scheduler as if this PR was just checked
	scheduler.SeedLastChecked(stalePR.URL(), now)

	// Act
	result := scheduler.DeterminePRsToCheckAt(now, []*pullrequest.PullRequest{stalePR})

	// Assert
	assert.Empty(t, result.PRsToCheck, "seeded PR should be treated as recently checked")
	assert.Equal(t, 1, result.SkippedCount)
}
