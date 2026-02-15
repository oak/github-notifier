package pullrequest

import (
	"time"
)

// PullRequestRepository is the port for fetching pull requests from external sources
//
//nolint:revive // keeping full name for clarity in this context
type PullRequestRepository interface {
	// FetchRequestedReviews fetches PRs where the user is requested to review
	FetchRequestedReviews() ([]*PullRequest, error)

	// FetchUserCreated fetches PRs created by the user
	FetchUserCreated() ([]*PullRequest, error)

	// EnrichWithActivities populates PRs with their activities since the given time
	// This modifies the aggregate by calling AddActivity through the proper aggregate methods
	EnrichWithActivities(prs []*PullRequest, since time.Time) error

	// AuthenticatedUser returns the login of the authenticated user.
	// Used to filter self-authored activities in notifications and tracking.
	AuthenticatedUser() string
}

// SeenRepository is the port for persisting seen pull requests
type SeenRepository interface {
	// MarkAsSeen marks a PR as seen
	MarkAsSeen(id PRIdentifier) error

	// UnmarkAsSeen marks a PR as unseen (e.g., when new activity occurs)
	UnmarkAsSeen(id PRIdentifier) error

	// HasBeenSeen checks if a PR has been seen
	HasBeenSeen(id PRIdentifier) bool

	// IsEmpty returns true if no PRs have been marked as seen yet
	IsEmpty() bool

	// Clear removes all seen PR records
	Clear() error
}
