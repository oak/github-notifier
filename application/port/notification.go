package port

import "github.com/oak3/github-notifier/domain/pullrequest"

// PRNotificationData represents the data for a single PR notification
type PRNotificationData struct {
	PullRequest   *pullrequest.PullRequest
	IsNew         bool
	Activities    []ActivityInfo
	StatusChanges []StatusChange
	ReviewChanges []ReviewChangeInfo
}

// ActivityInfo holds information about activities
type ActivityInfo struct {
	Type  pullrequest.ActivityType
	Count int
}

// StatusChange holds information about status changes
type StatusChange struct {
	EventType pullrequest.StatusChangeType
}

// ReviewChangeInfo holds information about a review state change
type ReviewChangeInfo struct {
	Reviewer string
	State    pullrequest.ReviewState
}

// NotificationPort is the port for sending notifications
type NotificationPort interface {
	// NotifyPullRequests sends grouped notifications for pull requests
	NotifyPullRequests(notifications []*PRNotificationData) error

	// NotifyMessage sends a simple text notification (e.g., setup instructions)
	NotifyMessage(title, message string) error

	// SupportsClickActions returns true if this adapter supports click actions
	SupportsClickActions() bool
}
