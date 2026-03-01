package pullrequest

import (
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
