package pullrequest

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Review represents a single reviewer's latest review state on a pull request
type Review struct {
	reviewer    Author
	state       ReviewState
	submittedAt time.Time
}

// NewReview creates a new review value object
func NewReview(reviewer Author, state ReviewState, submittedAt time.Time) *Review {
	return &Review{
		reviewer:    reviewer,
		state:       state,
		submittedAt: submittedAt,
	}
}

// Reviewer returns the reviewer
func (r *Review) Reviewer() Author {
	return r.reviewer
}

// State returns the review state
func (r *Review) State() ReviewState {
	return r.state
}

// SubmittedAt returns when the review was submitted
func (r *Review) SubmittedAt() time.Time {
	return r.submittedAt
}

// ReviewSummary provides a formatted summary of all reviews on a PR
type ReviewSummary struct {
	reviews map[string]*Review // reviewer login -> latest review
}

// NewReviewSummary creates a new review summary from a map of reviews
func NewReviewSummary(reviews map[string]*Review) *ReviewSummary {
	return &ReviewSummary{reviews: reviews}
}

// IsEmpty returns true if there are no reviews
func (rs *ReviewSummary) IsEmpty() bool {
	return len(rs.reviews) == 0
}

// FormatForMenu returns a formatted string for display in the system tray menu.
// Groups reviewers by state:
//   - (✅ Joe, Mike | ❌ Alice) — approved and changes requested
//   - (✅ Joe) — only approved
//   - (💬 Bob) — only commented
func (rs *ReviewSummary) FormatForMenu() string {
	if rs.IsEmpty() {
		return ""
	}

	// Group reviewers by state (only show approved and changes_requested in compact view)
	approved := rs.reviewersByState(ReviewStateApproved)
	changesRequested := rs.reviewersByState(ReviewStateChangesRequested)
	commented := rs.reviewersByState(ReviewStateCommented)

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

// reviewersByState returns sorted reviewer logins for a given state
func (rs *ReviewSummary) reviewersByState(state ReviewState) []string {
	var logins []string
	for login, review := range rs.reviews {
		if review.State() == state {
			logins = append(logins, login)
		}
	}
	return logins
}

// Reviews returns the individual reviews
func (rs *ReviewSummary) Reviews() map[string]*Review {
	// Return a copy
	result := make(map[string]*Review, len(rs.reviews))
	for k, v := range rs.reviews {
		result[k] = v
	}
	return result
}
