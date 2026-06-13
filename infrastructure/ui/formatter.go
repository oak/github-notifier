package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oak/github-notifier/domain/pullrequest"
)

// FormatReviewSummaryForMenu returns a formatted string for display in the
// system tray menu. Groups reviewers by state:
//   - (✅ Joe, Mike | ❌ Alice) — approved and changes requested
//   - (✅ Joe) — only approved
//   - (💬 Bob) — only commented
func FormatReviewSummaryForMenu(summary *pullrequest.ReviewSummary) string {
	if summary.IsEmpty() {
		return ""
	}

	approved := summary.ReviewersByState(pullrequest.ReviewStateApproved)
	changesRequested := summary.ReviewersByState(pullrequest.ReviewStateChangesRequested)
	commented := summary.ReviewersByState(pullrequest.ReviewStateCommented)

	var parts []string

	if len(approved) > 0 {
		sort.Strings(approved)
		parts = append(parts, fmt.Sprintf("✅ %s", strings.Join(approved, ", ")))
	}

	if len(commented) > 0 {
		sort.Strings(commented)
		parts = append(parts, fmt.Sprintf("💬 %s", strings.Join(commented, ", ")))
	}

	if len(changesRequested) > 0 {
		sort.Strings(changesRequested)
		parts = append(parts, fmt.Sprintf("❌ %s", strings.Join(changesRequested, ", ")))
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf("(%s)", strings.Join(parts, " | "))
}
