package pullrequest

import (
	"fmt"
	"time"
)

// Review ----------------------------------------------------------------------------

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

// ReviewSummary ------------------------------------------------------------

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

// ReviewersByState returns reviewer logins for a given state
func (rs *ReviewSummary) ReviewersByState(state ReviewState) []string {
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

// ReviewState represents the state of a pull request review
type ReviewState int

const (
	// ReviewStateApproved indicates the reviewer approved the PR
	ReviewStateApproved ReviewState = iota
	// ReviewStateChangesRequested indicates the reviewer requested changes
	ReviewStateChangesRequested
	// ReviewStateCommented indicates the reviewer left a comment without approving or requesting changes
	ReviewStateCommented
	// ReviewStateDismissed indicates a previous review was dismissed
	ReviewStateDismissed
)

// String returns a string representation of the review state
func (rs ReviewState) String() string {
	switch rs {
	case ReviewStateApproved:
		return "approved"
	case ReviewStateChangesRequested:
		return "changes_requested"
	case ReviewStateCommented:
		return "commented"
	case ReviewStateDismissed:
		return "dismissed"
	default:
		return "unknown"
	}
}

// Emoji returns a display emoji for the review state
func (rs ReviewState) Emoji() string {
	switch rs {
	case ReviewStateApproved:
		return "✅"
	case ReviewStateChangesRequested:
		return "❌"
	case ReviewStateCommented:
		return "💬"
	case ReviewStateDismissed:
		return "🚫"
	default:
		return "?"
	}
}

// Label returns a human-readable label for the review state
func (rs ReviewState) Label() string {
	switch rs {
	case ReviewStateApproved:
		return "approved"
	case ReviewStateChangesRequested:
		return "requested changes"
	case ReviewStateCommented:
		return "commented"
	case ReviewStateDismissed:
		return "dismissed"
	default:
		return "unknown"
	}
}

// MarshalText implements encoding.TextMarshaler so ReviewState serialises as a
// stable lowercase string in JSON and other text-based formats.
func (rs ReviewState) MarshalText() ([]byte, error) {
	return []byte(rs.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (rs *ReviewState) UnmarshalText(text []byte) error {
	switch string(text) {
	case "approved":
		*rs = ReviewStateApproved
	case "changes_requested":
		*rs = ReviewStateChangesRequested
	case "commented":
		*rs = ReviewStateCommented
	case "dismissed":
		*rs = ReviewStateDismissed
	default:
		return fmt.Errorf("unknown review state %q", string(text))
	}
	return nil
}
