package events

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// NotificationAggregator batches notifications and groups them by PR
type NotificationAggregator struct {
	mu                sync.Mutex
	pendingEvents     map[string]*port.PRNotificationData // Key: PR URL
	flushInterval     time.Duration
	onFlush           func(notifications []*port.PRNotificationData)
	flushTimer        *time.Timer
	stopCh            chan struct{}
	authenticatedUser string                    // GitHub login — used to filter self-authored activities
	ignoreConfig      *pullrequest.IgnoreConfig // loaded ignore config, may be nil
}

// NewNotificationAggregator creates a new notification aggregator.
// authenticatedUser is the GitHub login of the current user; activities authored by
// this user are filtered out (they are domain facts, but not worth notifying about).
// ignoreConfig may be nil if no ignore.yaml exists yet.
func NewNotificationAggregator(flushInterval time.Duration, onFlush func(notifications []*port.PRNotificationData), authenticatedUser string, ignoreConfig *pullrequest.IgnoreConfig) *NotificationAggregator {
	return &NotificationAggregator{
		pendingEvents:     make(map[string]*port.PRNotificationData),
		flushInterval:     flushInterval,
		onFlush:           onFlush,
		stopCh:            make(chan struct{}),
		authenticatedUser: authenticatedUser,
		ignoreConfig:      ignoreConfig,
	}
}

// UpdateIgnoreConfig atomically replaces the active ignore config.
// Safe to call from any goroutine.
func (a *NotificationAggregator) UpdateIgnoreConfig(cfg *pullrequest.IgnoreConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ignoreConfig = cfg
}

// AddEvent adds an event to the aggregator. It is a thin concurrency wrapper
// around the pure accumulateEvent fold function.
func (a *NotificationAggregator) AddEvent(event pullrequest.Event) {
	// If ignore config is loaded, filter event before aggregation
	if a.ignoreConfig != nil {
		repoName := ""
		author := ""
		eventName := event.Name()
		eventDetail := ""
		switch e := event.(type) {
		case *pullrequest.NewPullRequestDetected:
			repoName = e.Repository.NameWithOwner()
			author = e.Author.Login()
		case *pullrequest.ActivityDetected:
			repoName = e.Repository.NameWithOwner()
			if e.Activity != nil {
				author = e.Activity.Author().Login()
				eventDetail = string(e.Activity.Type())
			}
		case *pullrequest.ReviewStateChanged:
			repoName = e.Repository.NameWithOwner()
			author = e.Reviewer.Login()
			eventDetail = e.State.String()
		case *pullrequest.Merged:
			repoName = e.Repository.NameWithOwner()
			author = e.PullRequest.Author().Login()
		case *pullrequest.Closed:
			repoName = e.Repository.NameWithOwner()
			author = e.PullRequest.Author().Login()
		case *pullrequest.PipelineStatusChanged:
			repoName = e.Repository.NameWithOwner()
			author = e.PullRequest.Author().Login()
			eventDetail = e.NewStatus.String()
		}
		if pullrequest.ActivityIgnoreFilter(a.ignoreConfig, repoName, eventName, author, eventDetail) {
			log.Debug().
				Str("event", eventName).
				Str("repo", repoName).
				Str("author", author).
				Str("detail", eventDetail).
				Msg("event suppressed by ignore rule")
			return
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pendingEvents = accumulateEvent(a.pendingEvents, event, a.authenticatedUser)
	a.resetFlushTimer()
}

// accumulateEvent is a pure fold function that incorporates a single domain event
// into the current per-PR notification batch. It has no I/O, no mutex, and no
// timer dependency, which makes it directly unit-testable without goroutines.
//
// The caller is responsible for synchronisation (see AddEvent).
func accumulateEvent(
	batch map[string]*port.PRNotificationData,
	event pullrequest.Event,
	authenticatedUser string,
) map[string]*port.PRNotificationData {
	switch e := event.(type) {
	case *pullrequest.NewPullRequestDetected:
		url := e.PullRequestID.URL()
		notification, exists := batch[url]
		if !exists {
			notification = &port.PRNotificationData{
				PullRequest: e.PullRequest,
				Activities:  []port.ActivityInfo{},
			}
			batch[url] = notification
		}
		notification.IsNew = true

	case *pullrequest.ActivityDetected:
		// Filter out self-authored activities — domain facts, not notification-worthy.
		if authenticatedUser != "" && e.Activity.Author().Login() == authenticatedUser {
			break
		}
		url := e.PullRequestID.URL()
		notification, exists := batch[url]
		if !exists {
			notification = &port.PRNotificationData{
				PullRequest: e.PullRequest,
				Activities:  []port.ActivityInfo{},
			}
			batch[url] = notification
		}
		activityType := e.Activity.Type()
		found := false
		for i, existing := range notification.Activities {
			if existing.Type == activityType {
				notification.Activities[i].Count++
				found = true
				break
			}
		}
		if !found {
			notification.Activities = append(notification.Activities, port.ActivityInfo{
				Type:  activityType,
				Count: 1,
			})
		}

	case *pullrequest.ReviewStateChanged:
		// Filter out self-authored review state changes — not notification-worthy.
		if authenticatedUser != "" && e.Reviewer.Login() == authenticatedUser {
			break
		}
		url := e.PullRequestID.URL()
		notification, exists := batch[url]
		if !exists {
			notification = &port.PRNotificationData{
				PullRequest:   e.PullRequest,
				Activities:    []port.ActivityInfo{},
				ReviewChanges: []port.ReviewChangeInfo{},
			}
			batch[url] = notification
		}
		notification.ReviewChanges = append(notification.ReviewChanges, port.ReviewChangeInfo{
			Reviewer: e.Reviewer.Login(),
			State:    e.State,
		})

	case *pullrequest.Merged:
		url := e.PullRequestID.URL()
		notification, exists := batch[url]
		if !exists {
			notification = &port.PRNotificationData{
				PullRequest:   e.PullRequest,
				Activities:    []port.ActivityInfo{},
				StatusChanges: []port.StatusChange{},
			}
			batch[url] = notification
		}
		notification.StatusChanges = append(notification.StatusChanges, port.StatusChange{
			EventType: pullrequest.StatusChangeMerged,
		})

	case *pullrequest.Closed:
		url := e.PullRequestID.URL()
		notification, exists := batch[url]
		if !exists {
			notification = &port.PRNotificationData{
				PullRequest:   e.PullRequest,
				Activities:    []port.ActivityInfo{},
				StatusChanges: []port.StatusChange{},
			}
			batch[url] = notification
		}
		notification.StatusChanges = append(notification.StatusChanges, port.StatusChange{
			EventType: pullrequest.StatusChangeClosed,
		})

	case *pullrequest.PipelineStatusChanged:
		url := e.PullRequestID.URL()
		notification, exists := batch[url]
		if !exists {
			notification = &port.PRNotificationData{
				PullRequest: e.PullRequest,
				Activities:  []port.ActivityInfo{},
			}
			batch[url] = notification
		}
		// Keep only the latest transition: preserve OldStatus from the first event
		// in this flush window so the user sees the full transition.
		if notification.PipelineChange == nil {
			notification.PipelineChange = &port.PipelineStatusChange{
				OldStatus: e.OldStatus,
				NewStatus: e.NewStatus,
			}
		} else {
			// Update only the NewStatus — the OldStatus stays as the starting point.
			notification.PipelineChange.NewStatus = e.NewStatus
		}
	}

	return batch
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
	notifications := make([]*port.PRNotificationData, 0, len(a.pendingEvents))
	for _, notification := range a.pendingEvents {
		notifications = append(notifications, notification)
	}

	// Clear the pending events
	a.pendingEvents = make(map[string]*port.PRNotificationData)

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
