package pullrequest

// Repository is the port for fetching pull requests from external sources
type Repository interface {
	// FetchRequestedReviews fetches PRs where the user is requested to review
	FetchRequestedReviews() ([]*PullRequest, error)

	// FetchUserCreated fetches PRs created by the user
	FetchUserCreated() ([]*PullRequest, error)
}
