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
	jsonrepo "github.com/oak3/github-notifier/infrastructure/persistence/json"
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
	trackingRepo := memory.NewPRTrackingRepository()
	trackingService := pullrequest.NewTrackingService(seenRepo)

	// Setup event infrastructure
	eventBus := events.NewInMemoryEventBus()

	// Register event handlers
	notificationHandler := events.NewNotificationEventHandler(notifications, githubAdapter.AuthenticatedUser())
	trackingHandler := events.NewTrackingEventHandler(trackingService)

	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventReviewStateChanged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventMerged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventClosed, notificationHandler)
	eventBus.Subscribe(pullrequest.EventPipelineStatusChanged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventReviewStateChanged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventMerged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventClosed, trackingHandler)
	eventBus.Subscribe(pullrequest.EventPipelineStatusChanged, trackingHandler)

	// Setup domain services
	prFilter := pullrequest.NewDraftFilter(false) // Don't include drafts by default
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
		trackingRepo,
		trackingService,
		prFilter,
		eventBus,
	)

	detectClosedPRsUseCase := usecase.NewDetectClosedPullRequestsUseCase(
		githubAdapter,
		trackingRepo,
		eventBus,
	)

	trackActivityUseCase := usecase.NewTrackPullRequestActivityUseCase(
		githubAdapter,
		trackingRepo,
		activityScheduler,
		trackingService,
		eventBus,
		githubAdapter.AuthenticatedUser(),
	)

	updateDisplayUseCase := usecase.NewUpdatePullRequestDisplayUseCase(
		menuAdapter,
		trackingService,
	)

	// Create orchestrator
	orchestrator := application.NewPullRequestOrchestrator(
		initializeUseCase,
		checkNewPRsUseCase,
		detectClosedPRsUseCase,
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

// SetupSuiteFromStateFile creates a TestSuite backed by a JSON StateRepository
// at the given path.  Two successive calls with the same path simulate a
// process restart: the second instance loads all state that the first one saved.
// The mock GitHub server is NOT shared between calls; the caller must configure
// both instances independently (or pass the URL directly via the mock returned
// from the first call).
func SetupSuiteFromStateFile(t *testing.T, stateFilePath string) *TestSuite {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	mockGitHub := SetupMockGitHubServer()

	notifications := NewSpyNotificationAdapter()
	menuAdapter := NewSpyUIAdapter()

	githubAdapter := github.NewAdapterWithURL(mockGitHub.URL)
	stateRepo := jsonrepo.NewStateRepository(stateFilePath)
	trackingService := pullrequest.NewTrackingService(stateRepo)

	eventBus := events.NewInMemoryEventBus()

	notificationHandler := events.NewNotificationEventHandler(notifications, githubAdapter.AuthenticatedUser())
	trackingHandler := events.NewTrackingEventHandler(trackingService)

	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventReviewStateChanged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventMerged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventClosed, notificationHandler)
	eventBus.Subscribe(pullrequest.EventPipelineStatusChanged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventReviewStateChanged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventMerged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventClosed, trackingHandler)
	eventBus.Subscribe(pullrequest.EventPipelineStatusChanged, trackingHandler)

	prFilter := pullrequest.NewDraftFilter(false)
	activityScheduler := pullrequest.NewActivityCheckScheduler(72, 15)

	initializeUseCase := usecase.NewInitializeFirstCheckUseCase(githubAdapter, trackingService, prFilter, menuAdapter)
	checkNewPRsUseCase := usecase.NewCheckNewPullRequestsUseCase(githubAdapter, stateRepo, trackingService, prFilter, eventBus)
	detectClosedPRsUseCase := usecase.NewDetectClosedPullRequestsUseCase(githubAdapter, stateRepo, eventBus)
	trackActivityUseCase := usecase.NewTrackPullRequestActivityUseCase(githubAdapter, stateRepo, activityScheduler, trackingService, eventBus, githubAdapter.AuthenticatedUser())
	updateDisplayUseCase := usecase.NewUpdatePullRequestDisplayUseCase(menuAdapter, trackingService)

	orchestrator := application.NewPullRequestOrchestrator(
		initializeUseCase,
		checkNewPRsUseCase,
		detectClosedPRsUseCase,
		trackActivityUseCase,
		updateDisplayUseCase,
		true,
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

// SetupSuiteOnMockServer creates a TestSuite connected to an existing
// MockGitHubServer.  Useful for restart tests where the second "process" must
// talk to the same mock server as the first one.
func SetupSuiteOnMockServer(t *testing.T, mockGitHub *MockGitHubServer, stateFilePath string) *TestSuite {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	notifications := NewSpyNotificationAdapter()
	menuAdapter := NewSpyUIAdapter()

	githubAdapter := github.NewAdapterWithURL(mockGitHub.URL)
	stateRepo := jsonrepo.NewStateRepository(stateFilePath)
	trackingService := pullrequest.NewTrackingService(stateRepo)

	eventBus := events.NewInMemoryEventBus()

	notificationHandler := events.NewNotificationEventHandler(notifications, githubAdapter.AuthenticatedUser())
	trackingHandler := events.NewTrackingEventHandler(trackingService)

	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventReviewStateChanged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventMerged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventClosed, notificationHandler)
	eventBus.Subscribe(pullrequest.EventPipelineStatusChanged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventReviewStateChanged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventMerged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventClosed, trackingHandler)
	eventBus.Subscribe(pullrequest.EventPipelineStatusChanged, trackingHandler)

	prFilter := pullrequest.NewDraftFilter(false)
	activityScheduler := pullrequest.NewActivityCheckScheduler(72, 15)

	initializeUseCase := usecase.NewInitializeFirstCheckUseCase(githubAdapter, trackingService, prFilter, menuAdapter)
	checkNewPRsUseCase := usecase.NewCheckNewPullRequestsUseCase(githubAdapter, stateRepo, trackingService, prFilter, eventBus)
	detectClosedPRsUseCase := usecase.NewDetectClosedPullRequestsUseCase(githubAdapter, stateRepo, eventBus)
	trackActivityUseCase := usecase.NewTrackPullRequestActivityUseCase(githubAdapter, stateRepo, activityScheduler, trackingService, eventBus, githubAdapter.AuthenticatedUser())
	updateDisplayUseCase := usecase.NewUpdatePullRequestDisplayUseCase(menuAdapter, trackingService)

	orchestrator := application.NewPullRequestOrchestrator(
		initializeUseCase,
		checkNewPRsUseCase,
		detectClosedPRsUseCase,
		trackActivityUseCase,
		updateDisplayUseCase,
		true,
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
