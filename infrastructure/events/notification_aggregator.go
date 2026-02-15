package events

import (
	"sync"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// PRNotification represents aggregated notification data for a PR
type PRNotification struct {
	PullRequest   *pullrequest.PullRequest
	IsNew         bool
	Activities    []ActivityInfo
	StatusChanges []StatusChange
}

// ActivityInfo holds information about a specific activity
type ActivityInfo struct {
	Type  pullrequest.ActivityType
	Count int
}

// StatusChange holds information about status changes
type StatusChange struct {
	EventType pullrequest.StatusChangeType
}

// NotificationAggregator batches notifications and groups them by PR
type NotificationAggregator struct {
	mu                sync.Mutex
	pendingEvents     map[string]*PRNotification // Key: PR URL
	flushInterval     time.Duration
	onFlush           func(notifications []*PRNotification)
	flushTimer        *time.Timer
	stopCh            chan struct{}
	authenticatedUser string // GitHub login — used to filter self-authored activities
}

// NewNotificationAggregator creates a new notification aggregator.
// authenticatedUser is the GitHub login of the current user; activities authored by
// this user are filtered out (they are domain facts, but not worth notifying about).
func NewNotificationAggregator(flushInterval time.Duration, onFlush func(notifications []*PRNotification), authenticatedUser string) *NotificationAggregator {
	return &NotificationAggregator{
		pendingEvents:     make(map[string]*PRNotification),
		flushInterval:     flushInterval,
		onFlush:           onFlush,
		stopCh:            make(chan struct{}),
		authenticatedUser: authenticatedUser,
	}
}

// AddEvent adds an event to the aggregator
func (a *NotificationAggregator) AddEvent(event pullrequest.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch e := event.(type) {
	case *pullrequest.NewPullRequestDetected:
		a.addNewPREvent(e)
	case *pullrequest.ActivityDetected:
		a.addActivityEvent(e)
	case *pullrequest.Merged:
		a.addMergedEvent(e)
	case *pullrequest.Closed:
		a.addClosedEvent(e)
	}

	// Reset or start the flush timer
	a.resetFlushTimer()
}

// addNewPREvent adds a new PR detected event
func (a *NotificationAggregator) addNewPREvent(event *pullrequest.NewPullRequestDetected) {
	url := event.PullRequestID.URL()

	notification, exists := a.pendingEvents[url]
	if !exists {
		notification = &PRNotification{
			PullRequest: event.PullRequest,
			Activities:  []ActivityInfo{},
		}
		a.pendingEvents[url] = notification
	}

	notification.IsNew = true
}

// addActivityEvent adds activity to the existing PR notification.
// Filters out activities authored by the authenticated user — those are domain facts
// recorded by the aggregate, but not worth notifying the user about.
func (a *NotificationAggregator) addActivityEvent(event *pullrequest.ActivityDetected) {
	// Filter out self-authored activities — not notification-worthy
	if a.authenticatedUser != "" && event.Activity.Author().Login() == a.authenticatedUser {
		return
	}

	url := event.PullRequestID.URL()

	notification, exists := a.pendingEvents[url]
	if !exists {
		notification = &PRNotification{
			PullRequest: event.PullRequest,
			Activities:  []ActivityInfo{},
		}
		a.pendingEvents[url] = notification
	}

	activityType := event.Activity.Type()

	// Check if this activity type already exists
	found := false
	for i, existing := range notification.Activities {
		if existing.Type == activityType {
			notification.Activities[i].Count++
			found = true
			break
		}
	}
	if !found {
		notification.Activities = append(notification.Activities, ActivityInfo{
			Type:  activityType,
			Count: 1,
		})
	}
}

// addMergedEvent adds a merged status change
func (a *NotificationAggregator) addMergedEvent(event *pullrequest.Merged) {
	url := event.PullRequestID.URL()

	notification, exists := a.pendingEvents[url]
	if !exists {
		// We don't have the full PR object here, so skip if not already tracked
		return
	}

	notification.StatusChanges = append(notification.StatusChanges, StatusChange{
		EventType: pullrequest.StatusChangeMerged,
	})
}

// addClosedEvent adds a closed status change
func (a *NotificationAggregator) addClosedEvent(event *pullrequest.Closed) {
	url := event.PullRequestID.URL()

	notification, exists := a.pendingEvents[url]
	if !exists {
		// We don't have the full PR object here, so skip if not already tracked
		return
	}

	notification.StatusChanges = append(notification.StatusChanges, StatusChange{
		EventType: pullrequest.StatusChangeClosed,
	})
}

// resetFlushTimer resets the flush timer
func (a *NotificationAggregator) resetFlushTimer() {
	if a.flushTimer != nil {
		a.flushTimer.Stop()
	}

	a.flushTimer = time.AfterFunc(a.flushInterval, func() {
		a.Flush()
	})
}

// Flush sends all pending notifications and clears the buffer
func (a *NotificationAggregator) Flush() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.pendingEvents) == 0 {
		return
	}

	// Collect all notifications
	notifications := make([]*PRNotification, 0, len(a.pendingEvents))
	for _, notification := range a.pendingEvents {
		notifications = append(notifications, notification)
	}

	// Clear the pending events
	a.pendingEvents = make(map[string]*PRNotification)

	// Call the flush callback
	if a.onFlush != nil {
		a.onFlush(notifications)
	}
}

// Stop stops the aggregator and flushes any pending notifications
func (a *NotificationAggregator) Stop() {
	close(a.stopCh)
	a.Flush()
}
