package pullrequest

import (
	"time"
)

// PullRequestRepository is the port for fetching pull requests from external sources
type PullRequestRepository interface {
	FetchRequestedReviews() ([]*PullRequest, error)

	FetchUserCreated() ([]*PullRequest, error)

	// FetchActivities fetches raw activity/enrichment facts for the given PRs
	// since the provided time. Adapters return data only; aggregate mutation and
	// domain event creation/publishing happen in the application/domain layers.
	FetchActivities(prs []*PullRequest, since time.Time) (map[string]PRActivityData, error)

	// FetchPRStatus fetches the current status of a specific PR (open, merged, closed).
	FetchPRStatus(owner, repo string, number int) (PRStatus, error)

	// AuthenticatedUser returns the login of the authenticated user.
	AuthenticatedUser() string
}

// PRActivityData contains fetch-only enrichment facts for a PR.
// It is keyed by PR URL in PullRequestRepository.FetchActivities results.
// Optional fields are represented as pointers so callers can distinguish
// between "not present in response" and a concrete value.
type PRActivityData struct {
	Activities     []*Activity
	HeadCommitSHA  *string
	PipelineStatus *PipelineStatus
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

	LoadAll() ([]*PullRequest, error)

	Update(pullRequest *PullRequest) error

	Save(pullRequests []*PullRequest) error

	IsEmpty() bool

	Clear() error
}
