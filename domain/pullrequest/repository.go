package pullrequest

import "time"

// PullRequestRepository is the port for fetching pull requests from external sources
type PullRequestRepository interface {
	// FetchRequestedReviews fetches PRs where the user is requested to review
	FetchRequestedReviews() ([]*PullRequest, error)

	// FetchUserCreated fetches PRs created by the user
	FetchUserCreated() ([]*PullRequest, error)

	// EnrichWithActivities populates PRs with their activities since the given time
	// This modifies the aggregate by calling AddActivity through the proper aggregate methods
	EnrichWithActivities(prs []*PullRequest, since time.Time) error
}
