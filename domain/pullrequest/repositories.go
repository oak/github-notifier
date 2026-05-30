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

	// EnrichWithActivities populates PRs with their activities since the given time.
	// Returns all domain events raised by the aggregate during enrichment
	// (ActivityDetected, PipelineStatusChanged, etc.) so callers can publish them
	// without needing to drain an internal event queue.
	EnrichWithActivities(prs []*PullRequest, since time.Time) ([]Event, error)

	// FetchPRStatus fetches the current status of a specific PR (open, merged, closed).
	// Used to determine the final status of PRs that have disappeared from the open PR list.
	FetchPRStatus(owner, repo string, number int) (PRStatus, error)

	// AuthenticatedUser returns the login of the authenticated user.
	// Used to filter self-authored activities in notifications and tracking.
	AuthenticatedUser() string
}

// PRTrackingRepository is the port for persisting the locally-tracked state of
// open pull requests across process restarts. It stores the identity and
// mutable fields that cannot be cheaply re-derived from the GitHub API each
// cycle (head commit SHA, pipeline status, last activity check timestamp,
// reviews, etc.).
//
// Adapters must treat Save as a full replacement of the stored set — it is not
// an upsert. Only open PRs are ever stored; closed/merged PRs are excluded
// before Save is called.
type PRTrackingRepository interface {
	// Save replaces the entire stored set with the provided snapshots.
	// Called after every successful check cycle with the current open PR set.
	Save(snapshots []PRStateSnapshot) error

	// LoadAll returns all previously saved snapshots.
	// Returns an empty slice (not an error) when no state has been saved yet.
	LoadAll() ([]PRStateSnapshot, error)

	// Clear removes all stored snapshots (e.g. on a hard reset).
	Clear() error
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
