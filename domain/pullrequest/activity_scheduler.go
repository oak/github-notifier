package pullrequest

import (
	"log"
	"time"
)

// ActivityCheckScheduler implements a two-tier scheduling strategy for checking PR activities
// Recent PRs (younger than threshold) are checked frequently
// Stale PRs (older than threshold) are checked less frequently
type ActivityCheckScheduler struct {
	recentThreshold    time.Duration // PRs younger than this are "recent"
	staleCheckInterval time.Duration // How often to check stale PRs
	lastCheckMap       map[string]time.Time
}

// NewActivityCheckScheduler creates a new activity check scheduler
func NewActivityCheckScheduler(recentThresholdHours int, staleCheckIntervalMin int) *ActivityCheckScheduler {
	return &ActivityCheckScheduler{
		recentThreshold:    time.Duration(recentThresholdHours) * time.Hour,
		staleCheckInterval: time.Duration(staleCheckIntervalMin) * time.Minute,
		lastCheckMap:       make(map[string]time.Time),
	}
}

// ScheduleResult contains the results of scheduling determination
type ScheduleResult struct {
	PRsToCheck   []*PullRequest
	RecentCount  int
	StaleCount   int
	SkippedCount int
}

// DeterminePRsToCheck implements the two-tier scheduling logic
// Returns which PRs should be checked for activity based on:
// - Recent PRs (age < recentThreshold): Always check
// - Stale PRs (age >= recentThreshold): Check only if staleCheckInterval has passed since last check
func (s *ActivityCheckScheduler) DeterminePRsToCheck(prs []*PullRequest) *ScheduleResult {
	result := &ScheduleResult{
		PRsToCheck: make([]*PullRequest, 0, len(prs)),
	}

	now := time.Now()

	for _, pr := range prs {
		prURL := pr.URL()
		prAge := now.Sub(pr.CreatedAt())
		lastCheck := s.lastCheckMap[prURL]

		shouldCheck := false

		if prAge < s.recentThreshold {
			// Recent PR: always check
			shouldCheck = true
			result.RecentCount++
		} else {
			// Stale PR: check only if enough time passed since last check
			timeSinceLastCheck := now.Sub(lastCheck)
			if timeSinceLastCheck >= s.staleCheckInterval {
				shouldCheck = true
				result.StaleCount++
			}
		}

		if shouldCheck {
			result.PRsToCheck = append(result.PRsToCheck, pr)
		}
	}

	result.SkippedCount = len(prs) - len(result.PRsToCheck)

	log.Printf("Activity scheduler: checking %d/%d PRs (%d recent < %dh, %d stale due for check, %d skipped)",
		len(result.PRsToCheck), len(prs), result.RecentCount,
		int(s.recentThreshold.Hours()), result.StaleCount, result.SkippedCount)

	return result
}

// MarkChecked records that the given PRs were checked at the current time
// This is used to track when stale PRs were last checked
func (s *ActivityCheckScheduler) MarkChecked(prs []*PullRequest) {
	now := time.Now()
	for _, pr := range prs {
		s.lastCheckMap[pr.URL()] = now
	}
}
