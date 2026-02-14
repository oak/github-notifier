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
	EventType string // "merged" or "closed"
}

// NotificationAggregator batches notifications and groups them by PR
type NotificationAggregator struct {
	mu            sync.Mutex
	pendingEvents map[string]*PRNotification // Key: PR URL
	flushInterval time.Duration
	onFlush       func(notifications []*PRNotification)
	flushTimer    *time.Timer
	stopCh        chan struct{}
}

// NewNotificationAggregator creates a new notification aggregator
func NewNotificationAggregator(flushInterval time.Duration, onFlush func(notifications []*PRNotification)) *NotificationAggregator {
	return &NotificationAggregator{
		pendingEvents: make(map[string]*PRNotification),
		flushInterval: flushInterval,
		onFlush:       onFlush,
		stopCh:        make(chan struct{}),
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

// addActivityEvent adds activity to the existing PR notification
func (a *NotificationAggregator) addActivityEvent(event *pullrequest.ActivityDetected) {
	url := event.PullRequestID.URL()

	notification, exists := a.pendingEvents[url]
	if !exists {
		notification = &PRNotification{
			PullRequest: event.PullRequest,
			Activities:  []ActivityInfo{},
		}
		a.pendingEvents[url] = notification
	}

	// Group activities by type and count them
	activityCounts := make(map[pullrequest.ActivityType]int)
	for _, activity := range event.Activities {
		activityCounts[activity.Type()]++
	}

	// Convert to activity info list
	for activityType, count := range activityCounts {
		// Check if this activity type already exists
		found := false
		for i, existing := range notification.Activities {
			if existing.Type == activityType {
				notification.Activities[i].Count += count
				found = true
				break
			}
		}
		if !found {
			notification.Activities = append(notification.Activities, ActivityInfo{
				Type:  activityType,
				Count: count,
			})
		}
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
		EventType: "merged",
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
		EventType: "closed",
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
