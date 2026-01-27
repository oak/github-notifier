package tracking

import (
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Service is the domain service for tracking pull requests
type Service interface {
	// TrackPullRequest tracks a PR and returns true if it's new (not seen before)
	TrackPullRequest(pr *pullrequest.PullRequest) bool

	// HasBeenSeen checks if a PR has been seen before
	HasBeenSeen(id pullrequest.PRIdentifier) bool

	// FindNewPullRequests identifies which PRs in the list are new
	FindNewPullRequests(prs []*pullrequest.PullRequest) []*pullrequest.PullRequest

	// MarkPullRequestsAsSeen marks a list of PRs as seen
	MarkPullRequestsAsSeen(prs []*pullrequest.PullRequest)

	// MarkPullRequestAsUnseen marks a PR as unseen (triggers notifications again)
	MarkPullRequestAsUnseen(pr *pullrequest.PullRequest) error

	// IsEmpty returns true if no PRs have been tracked yet
	IsEmpty() bool
}
