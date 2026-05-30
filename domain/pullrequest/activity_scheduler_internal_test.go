package pullrequest

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCfg(recentHours, staleMinutes int) schedulerConfig {
	return schedulerConfig{
		RecentThreshold:    time.Duration(recentHours) * time.Hour,
		StaleCheckInterval: time.Duration(staleMinutes) * time.Minute,
	}
}

func newInternalTestPR(t *testing.T, number int, url string, createdAt time.Time) *PullRequest {
	t.Helper()

	repo, err := NewRepository("oak3/github-notifier")
	require.NoError(t, err)

	author, err := NewAuthor("oak3")
	require.NoError(t, err)

	pr, err := NewPullRequest(url, number, "test pr", repo, author, createdAt, false)
	require.NoError(t, err)

	return pr
}

func prWithAge(t *testing.T, number int, now time.Time, age time.Duration) *PullRequest {
	t.Helper()
	url := fmt.Sprintf("https://github.com/oak3/github-notifier/pull/%d", number)
	return newInternalTestPR(t, number, url, now.Add(-age))
}

// ─── determineChecks ─────────────────────────────────────────────────────────

func TestDetermineChecks_AllRecent(t *testing.T) {
	now := time.Now()
	cfg := newCfg(48, 15)
	prs := []*PullRequest{
		prWithAge(t, 1, now, 1*time.Hour),
		prWithAge(t, 2, now, 2*time.Hour),
		prWithAge(t, 3, now, 3*time.Hour),
	}

	toCheck, recentN, staleN, skippedN := determineChecks(prs, nil, cfg, now)

	assert.Len(t, toCheck, 3)
	assert.Equal(t, 3, recentN)
	assert.Equal(t, 0, staleN)
	assert.Equal(t, 0, skippedN)
}

func TestDetermineChecks_StaleRecentlyChecked(t *testing.T) {
	now := time.Now()
	cfg := newCfg(48, 15)
	pr := prWithAge(t, 1, now, 72*time.Hour)

	lastChecked := map[string]time.Time{pr.URL(): now}

	toCheck, recentN, staleN, skippedN := determineChecks([]*PullRequest{pr}, lastChecked, cfg, now)

	assert.Empty(t, toCheck)
	assert.Equal(t, 0, recentN)
	assert.Equal(t, 0, staleN)
	assert.Equal(t, 1, skippedN)
}

func TestDetermineChecks_RecentThresholdBoundary(t *testing.T) {
	now := time.Now()
	cfg := newCfg(48, 15)

	prAtBoundary := prWithAge(t, 1, now, 48*time.Hour)
	prJustBefore := prWithAge(t, 2, now, 47*time.Hour+59*time.Minute)
	prJustAfter := prWithAge(t, 3, now, 48*time.Hour+1*time.Minute)

	toCheck, recentN, staleN, _ := determineChecks([]*PullRequest{prAtBoundary, prJustBefore, prJustAfter}, nil, cfg, now)

	assert.Len(t, toCheck, 3)
	assert.Equal(t, 1, recentN)
	assert.Equal(t, 2, staleN)
}

func TestDetermineChecks_TableDriven(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name             string
		recentHours      int
		staleMinutes     int
		prAges           []time.Duration
		lastCheckedIdxs  []int
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newCfg(tt.recentHours, tt.staleMinutes)
			prs := make([]*PullRequest, len(tt.prAges))
			for i, age := range tt.prAges {
				prs[i] = prWithAge(t, i+1, now, age)
			}

			lastChecked := make(map[string]time.Time, len(tt.lastCheckedIdxs))
			for _, idx := range tt.lastCheckedIdxs {
				lastChecked[prs[idx].URL()] = now
			}

			toCheck, recentN, staleN, skippedN := determineChecks(prs, lastChecked, cfg, now)

			assert.Len(t, toCheck, tt.expectedTotal)
			assert.Equal(t, tt.expectedRecentN, recentN)
			assert.Equal(t, tt.expectedStaleN, staleN)
			assert.Equal(t, tt.expectedSkippedN, skippedN)
		})
	}
}

// ─── determinePRsToCheckAt ───────────────────────────────────────────────────

func TestActivityCheckScheduler_DeterminePRsToCheckAt_DelegatesCorrectly(t *testing.T) {
	now := time.Now()
	scheduler := NewActivityCheckScheduler(48, 15)
	stalePR := prWithAge(t, 1, now, 72*time.Hour)
	prs := []*PullRequest{stalePR}

	result1 := scheduler.determinePRsToCheckAt(now, prs)
	assert.Len(t, result1.PRsToCheck, 1)

	scheduler.MarkCheckedAt(now, prs)
	result2 := scheduler.determinePRsToCheckAt(now, prs)

	assert.Empty(t, result2.PRsToCheck)
	assert.Equal(t, 1, result2.SkippedCount)
}

func TestActivityCheckScheduler_DeterminePRsToCheckAt_SeedLastChecked(t *testing.T) {
	now := time.Now()
	scheduler := NewActivityCheckScheduler(48, 15)
	stalePR := prWithAge(t, 1, now, 72*time.Hour)

	scheduler.SeedLastChecked(stalePR.URL(), now)
	result := scheduler.determinePRsToCheckAt(now, []*PullRequest{stalePR})

	assert.Empty(t, result.PRsToCheck)
	assert.Equal(t, 1, result.SkippedCount)
}

// ─── recordChecked ────────────────────────────────────────────────────────────

func TestRecordChecked_AddsEntries(t *testing.T) {
	now := time.Now()
	pr := newInternalTestPR(t, 1, "https://github.com/oak3/github-notifier/pull/1", now.Add(-72*time.Hour))

	result := recordChecked(nil, []*PullRequest{pr}, now)

	assert.Len(t, result, 1)
	assert.Equal(t, now, result[pr.URL()])
}

func TestRecordChecked_DoesNotMutateInput(t *testing.T) {
	now := time.Now()
	pr1 := newInternalTestPR(t, 1, "https://github.com/oak3/github-notifier/pull/1", now.Add(-72*time.Hour))
	pr2 := newInternalTestPR(t, 2, "https://github.com/oak3/github-notifier/pull/2", now.Add(-96*time.Hour))

	original := map[string]time.Time{pr1.URL(): now.Add(-1 * time.Hour)}

	result := recordChecked(original, []*PullRequest{pr2}, now)

	assert.Len(t, original, 1, "input map must not be mutated")
	assert.Len(t, result, 2)
	assert.Equal(t, now.Add(-1*time.Hour), result[pr1.URL()], "existing entry must be preserved")
	assert.Equal(t, now, result[pr2.URL()], "new entry must be added")
}

func TestRecordChecked_OverwritesExistingEntry(t *testing.T) {
	now := time.Now()
	pr := newInternalTestPR(t, 1, "https://github.com/oak3/github-notifier/pull/1", now.Add(-72*time.Hour))
	old := now.Add(-1 * time.Hour)

	original := map[string]time.Time{pr.URL(): old}

	result := recordChecked(original, []*PullRequest{pr}, now)

	assert.Equal(t, now, result[pr.URL()], "timestamp must be updated to now")
}
