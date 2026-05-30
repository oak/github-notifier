package pullrequest

import (
	"time"
)

// schedulerConfig holds the immutable configuration for the two-tier scheduling strategy.
type schedulerConfig struct {
	RecentThreshold    time.Duration // PRs younger than this are "recent"
	StaleCheckInterval time.Duration // How often to check stale PRs
}

// determineChecks is a pure function that determines which PRs need activity checks.
// No I/O — same inputs always produce the same output.
// Returns the PRs to check plus per-tier counts (recent, stale, skipped).
//
// Two-tier logic:
//   - Recent PRs (age < cfg.RecentThreshold): always check
//   - Stale PRs (age >= cfg.RecentThreshold): check only if cfg.StaleCheckInterval has elapsed
//     since the last entry in lastChecked (a missing entry is treated as never checked)
func determineChecks(
	prs []*PullRequest,
	lastChecked map[string]time.Time,
	cfg schedulerConfig,
	now time.Time,
) (toCheck []*PullRequest, recentN, staleN, skippedN int) {
	toCheck = make([]*PullRequest, 0, len(prs))

	for _, pr := range prs {
		prAge := now.Sub(pr.CreatedAt())
		shouldCheck := false

		if prAge < cfg.RecentThreshold {
			// Recent PR: always check
			shouldCheck = true
			recentN++
		} else {
			// Stale PR: check only if enough time has passed since last check
			timeSinceLastCheck := now.Sub(lastChecked[pr.URL()])
			if timeSinceLastCheck >= cfg.StaleCheckInterval {
				shouldCheck = true
				staleN++
			}
		}

		if shouldCheck {
			toCheck = append(toCheck, pr)
		}
	}

	skippedN = len(prs) - len(toCheck)
	return
}

// recordChecked is a pure function that returns a new map with the given PRs
// recorded as checked at now. The input map is never mutated.
func recordChecked(
	lastChecked map[string]time.Time,
	prs []*PullRequest,
	now time.Time,
) map[string]time.Time {
	updated := make(map[string]time.Time, len(lastChecked)+len(prs))
	for k, v := range lastChecked {
		updated[k] = v
	}
	for _, pr := range prs {
		updated[pr.URL()] = now
	}
	return updated
}

// ActivityCheckScheduler is a thin stateful wrapper around the pure determineChecks
// and recordChecked functions. It owns lastCheckMap and supplies the current time
// so that call sites in use cases remain unchanged.
type ActivityCheckScheduler struct {
	cfg          schedulerConfig
	lastCheckMap map[string]time.Time
}

// NewActivityCheckScheduler creates a new activity check scheduler.
func NewActivityCheckScheduler(recentThresholdHours int, staleCheckIntervalMin int) *ActivityCheckScheduler {
	return &ActivityCheckScheduler{
		cfg: schedulerConfig{
			RecentThreshold:    time.Duration(recentThresholdHours) * time.Hour,
			StaleCheckInterval: time.Duration(staleCheckIntervalMin) * time.Minute,
		},
		lastCheckMap: make(map[string]time.Time),
	}
}

// scheduleResult contains the results of scheduling determination.
type scheduleResult struct {
	PRsToCheck   []*PullRequest
	RecentCount  int
	StaleCount   int
	SkippedCount int
}

// determinePRsToCheckAt returns which PRs should be checked for activity at the given time.
// Thin wrapper around determineChecks using the scheduler's owned state.
func (s *ActivityCheckScheduler) determinePRsToCheckAt(now time.Time, prs []*PullRequest) *scheduleResult {
	toCheck, recentN, staleN, skippedN := determineChecks(prs, s.lastCheckMap, s.cfg, now)
	return &scheduleResult{
		PRsToCheck:   toCheck,
		RecentCount:  recentN,
		StaleCount:   staleN,
		SkippedCount: skippedN,
	}
}

// DeterminePRsToCheck calls determinePRsToCheckAt with the current time.
func (s *ActivityCheckScheduler) DeterminePRsToCheck(prs []*PullRequest) *scheduleResult {
	return s.determinePRsToCheckAt(time.Now(), prs)
}

// MarkCheckedAt records that the given PRs were checked at the given time.
// Thin wrapper around recordChecked using the scheduler's owned state.
func (s *ActivityCheckScheduler) MarkCheckedAt(now time.Time, prs []*PullRequest) {
	s.lastCheckMap = recordChecked(s.lastCheckMap, prs, now)
}

// SeedLastChecked pre-populates the last-check timestamp for a single PR URL.
// Used to restore the scheduler's state after a process restart so that stale
// PRs that were recently checked are not immediately re-checked.
// Zero-value timestamps are ignored.
func (s *ActivityCheckScheduler) SeedLastChecked(url string, t time.Time) {
	if !t.IsZero() {
		s.lastCheckMap[url] = t
	}
}
