package port

import "github.com/oak3/github-notifier/domain/pullrequest"

// PRNotificationData represents the data for a single PR notification
type PRNotificationData struct {
	PullRequest   *pullrequest.PullRequest
	IsNew         bool
	Activities    []ActivityInfo
	StatusChanges []StatusChange
}

// ActivityInfo holds information about activities
type ActivityInfo struct {
	Type  pullrequest.ActivityType
	Count int
}

// StatusChange holds information about status changes
type StatusChange struct {
	EventType string // "merged" or "closed"
}

// NotificationPort is the port for sending notifications
type NotificationPort interface {
	// NotifyPullRequests sends grouped notifications for pull requests
	NotifyPullRequests(notifications []*PRNotificationData) error

	// SupportsClickActions returns true if this adapter supports click actions
	SupportsClickActions() bool
}
