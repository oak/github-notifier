//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oak3/github-notifier/application"
	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/events"
	"github.com/oak3/github-notifier/infrastructure/github"
	"github.com/oak3/github-notifier/infrastructure/persistence/memory"
)

// TestSuite holds the E2E test infrastructure
type TestSuite struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	mockGitHub          *MockGitHubServer
	orchestrator        *application.PullRequestOrchestrator
	notifications       *SpyNotificationAdapter
	menuAdapter         *SpyUIAdapter
	trackingService     *pullrequest.TrackingService
	notificationHandler *events.NotificationEventHandler
}

// SetupSuite creates a complete test environment
func SetupSuite(t *testing.T) *TestSuite {
	// Create test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	// Setup mock GitHub server
	mockGitHub := SetupMockGitHubServer()

	// Setup spy adapters
	notifications := NewSpyNotificationAdapter()
	menuAdapter := NewSpyUIAdapter()

	// Setup infrastructure
	githubAdapter := github.NewAdapterWithURL(mockGitHub.URL)
	seenRepo := memory.NewSeenPullRequestRepository()
	trackingService := pullrequest.NewTrackingService(seenRepo)

	// Setup event infrastructure
	eventBus := events.NewInMemoryEventBus()

	// Register event handlers
	notificationHandler := events.NewNotificationEventHandler(notifications)
	trackingHandler := events.NewTrackingEventHandler(trackingService)

	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventMerged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventClosed, notificationHandler)
	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventMerged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventClosed, trackingHandler)

	// Setup domain services
	prFilter := pullrequest.NewPRFilter(false) // Don't include drafts by default
	prClassifier := pullrequest.NewPRClassifier()
	activityScheduler := pullrequest.NewActivityCheckScheduler(
		72, // Recent PR threshold hours (default)
		15, // Stale PR check interval minutes (default)
	)

	// Initialize use cases
	initializeUseCase := usecase.NewInitializeFirstCheckUseCase(
		githubAdapter,
		trackingService,
		prFilter,
		menuAdapter,
	)

	checkNewPRsUseCase := usecase.NewCheckNewPullRequestsUseCase(
		githubAdapter,
		trackingService,
		prFilter,
		prClassifier,
		eventBus,
	)

	trackActivityUseCase := usecase.NewTrackPullRequestActivityUseCase(
		githubAdapter,
		activityScheduler,
		trackingService,
		eventBus,
	)

	updateDisplayUseCase := usecase.NewUpdatePullRequestDisplayUseCase(
		menuAdapter,
		trackingService,
	)

	// Create orchestrator
	orchestrator := application.NewPullRequestOrchestrator(
		initializeUseCase,
		checkNewPRsUseCase,
		trackActivityUseCase,
		updateDisplayUseCase,
		true, // Enable activity tracking
	)

	return &TestSuite{
		ctx:                 ctx,
		cancel:              cancel,
		mockGitHub:          mockGitHub,
		orchestrator:        orchestrator,
		notifications:       notifications,
		menuAdapter:         menuAdapter,
		trackingService:     trackingService,
		notificationHandler: notificationHandler,
	}
}

// Teardown cleans up the test environment
func (s *TestSuite) Teardown() {
	// Flush any pending notifications
	if s.notificationHandler != nil {
		s.notificationHandler.Stop()
	}
	if s.mockGitHub != nil {
		s.mockGitHub.Close()
	}
	if s.cancel != nil {
		s.cancel()
	}
}

// WaitForNotification waits for a notification with timeout
func (s *TestSuite) WaitForNotification(t *testing.T, timeout time.Duration) *CapturedNotification {
	t.Helper()

	deadline := time.Now().Add(timeout)
	initialCount := len(s.notifications.GetNotifications())

	for time.Now().Before(deadline) {
		notifs := s.notifications.GetNotifications()
		if len(notifs) > initialCount {
			return &notifs[len(notifs)-1]
		}
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// GetLatestNotification returns the most recent notification
func (s *TestSuite) GetLatestNotification() *CapturedNotification {
	notifs := s.notifications.GetNotifications()
	if len(notifs) == 0 {
		return nil
	}
	return &notifs[len(notifs)-1]
}

// ClearNotifications clears all captured notifications and flushes the aggregator
func (s *TestSuite) ClearNotifications() {
	// First flush any pending notifications from the aggregator to the spy
	if s.notificationHandler != nil {
		s.notificationHandler.Flush()
	}
	// Then clear the captured notifications from the spy
	s.notifications.Clear()
}

// FlushNotifications immediately flushes any pending notifications
func (s *TestSuite) FlushNotifications() {
	if s.notificationHandler != nil {
		s.notificationHandler.Flush()
	}
}
