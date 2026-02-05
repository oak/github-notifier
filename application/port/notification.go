package port

import "github.com/oak3/github-notifier/domain/pullrequest"

// NotificationPort is the port for sending notifications
type NotificationPort interface {
	// NotifyNewPullRequests sends a notification about new pull requests
	NotifyNewPullRequests(title string, prs []*pullrequest.PullRequest) error

	// SupportsClickActions returns true if this adapter supports click actions
	SupportsClickActions() bool
}
