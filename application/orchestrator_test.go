package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/oak3/github-notifier/application"
	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/mocks"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_ExecuteInitialCheck_FirstRun(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	// Create use cases
	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, trackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, trackingService, mockEventPublisher, "")
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, trackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations for first run
	mockSeenRepo.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	// 2 PRs should be marked as seen (MarkAsSeen called twice)
	mockSeenRepo.On("MarkAsSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(nil).Twice()
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), trackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		false, // activity tracking disabled
	)

	// Act
	err := orchestrator.ExecuteInitialCheck(context.Background())

	// Assert
	require.NoError(t, err)
	mockSeenRepo.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}

func TestOrchestrator_ExecuteInitialCheck_NotFirstRun(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, trackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, trackingService, mockEventPublisher, "")
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, trackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations for not first run (executes regular check)
	mockSeenRepo.On("IsEmpty").Return(false)
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	// All PRs have already been seen
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(true).Twice()
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), trackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		false,
	)

	// Act
	err := orchestrator.ExecuteInitialCheck(context.Background())

	// Assert
	require.NoError(t, err)
	mockSeenRepo.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}

func TestOrchestrator_ExecuteRegularCheck_WithoutActivityTracking(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, trackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, trackingService, mockEventPublisher, "")
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, trackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	// All PRs have already been seen
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(true).Twice()
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), trackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		false, // activity tracking disabled
	)

	// Act
	err := orchestrator.ExecuteRegularCheck(context.Background())

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
	// EnrichWithActivities should NOT be called when activity tracking is disabled
	mockPRRepo.AssertNotCalled(t, "EnrichWithActivities")
}

func TestOrchestrator_ExecuteRegularCheck_WithActivityTracking(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, trackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, trackingService, mockEventPublisher, "")
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, trackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	// All PRs have already been seen
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(true).Twice()
	mockPRRepo.On("EnrichWithActivities", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("time.Time")).Return(nil)
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), trackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		true, // activity tracking enabled
	)

	// Act
	err := orchestrator.ExecuteRegularCheck(context.Background())

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}

func TestOrchestrator_ExecuteRegularCheck_CheckNewPRsError(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, trackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, trackingService, mockEventPublisher, "")
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, trackingService)

	expectedErr := errors.New("github api error")

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(nil, expectedErr)

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		false,
	)

	// Act
	err := orchestrator.ExecuteRegularCheck(context.Background())

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	// Activity tracking and display should NOT be called
	mockPRRepo.AssertNotCalled(t, "EnrichWithActivities")
	mockUIPort.AssertNotCalled(t, "UpdateDisplay")
}

func TestOrchestrator_ExecuteRegularCheck_ActivityTrackingError_ContinuesWithDisplay(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, trackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, trackingService, mockEventPublisher, "")
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, trackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	// All PRs have already been seen
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(true).Twice()
	mockPRRepo.On("EnrichWithActivities", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("time.Time")).Return(errors.New("activity error"))
	// Display should still be called even if activity tracking fails
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), trackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		true, // activity tracking enabled
	)

	// Act
	err := orchestrator.ExecuteRegularCheck(context.Background())

	// Assert
	require.NoError(t, err) // Orchestrator doesn't fail on activity tracking error
	mockUIPort.AssertExpectations(t)
}
