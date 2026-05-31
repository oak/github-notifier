package pullrequest

import (
	"time"
)

// PullRequestRepository is the port for fetching pull requests from external sources
type PullRequestRepository interface {
	FetchRequestedReviews() ([]*PullRequest, error)

	FetchUserCreated() ([]*PullRequest, error)

	// EnrichWithActivities populates PRs with their activities since the given time.
	// Returns all domain events raised by the aggregate during enrichment
	// (ActivityDetected, PipelineStatusChanged, etc.) so callers can publish them
	// without needing to drain an internal event queue.
	EnrichWithActivities(prs []*PullRequest, since time.Time) ([]Event, error)

	// FetchPRStatus fetches the current status of a specific PR (open, merged, closed).
	FetchPRStatus(owner, repo string, number int) (PRStatus, error)

	// AuthenticatedUser returns the login of the authenticated user.
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
	Fetch(prIdentifier PRIdentifier) (*PullRequest, error)

	LoadAll() ([]PullRequest, error)

	Update(pullRequest PullRequest) error

	Save(pullRequests []PullRequest) error

	Clear() error
}
