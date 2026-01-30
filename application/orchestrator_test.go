package application_test

import (
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
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	// Create use cases
	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, mockTrackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, mockTrackingService, mockEventPublisher)
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations for first run
	mockTrackingService.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingService.On("MarkPullRequestsAsSeen", mock.AnythingOfType("[]*pullrequest.PullRequest")).Twice()
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		false, // activity tracking disabled
	)

	// Act
	err := orchestrator.ExecuteInitialCheck()

	// Assert
	require.NoError(t, err)
	mockTrackingService.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}

func TestOrchestrator_ExecuteInitialCheck_NotFirstRun(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, mockTrackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, mockTrackingService, mockEventPublisher)
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations for not first run (executes regular check)
	mockTrackingService.On("IsEmpty").Return(false)
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingService.On("FindNewPullRequests", mock.AnythingOfType("[]*pullrequest.PullRequest")).Return([]*pullrequest.PullRequest{}).Twice()
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		false,
	)

	// Act
	err := orchestrator.ExecuteInitialCheck()

	// Assert
	require.NoError(t, err)
	mockTrackingService.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}

func TestOrchestrator_ExecuteRegularCheck_WithoutActivityTracking(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, mockTrackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, mockTrackingService, mockEventPublisher)
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingService.On("FindNewPullRequests", mock.AnythingOfType("[]*pullrequest.PullRequest")).Return([]*pullrequest.PullRequest{}).Twice()
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		false, // activity tracking disabled
	)

	// Act
	err := orchestrator.ExecuteRegularCheck()

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
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, mockTrackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, mockTrackingService, mockEventPublisher)
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingService.On("FindNewPullRequests", mock.AnythingOfType("[]*pullrequest.PullRequest")).Return([]*pullrequest.PullRequest{}).Twice()
	mockPRRepo.On("EnrichWithActivities", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("time.Time")).Return(nil)
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		true, // activity tracking enabled
	)

	// Act
	err := orchestrator.ExecuteRegularCheck()

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}

func TestOrchestrator_ExecuteRegularCheck_CheckNewPRsError(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, mockTrackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, mockTrackingService, mockEventPublisher)
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingService)

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
	err := orchestrator.ExecuteRegularCheck()

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
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, mockTrackingService, prFilter, prClassifier, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, scheduler, mockTrackingService, mockEventPublisher)
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingService)

	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingService.On("FindNewPullRequests", mock.AnythingOfType("[]*pullrequest.PullRequest")).Return([]*pullrequest.PullRequest{}).Twice()
	mockPRRepo.On("EnrichWithActivities", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("time.Time")).Return(errors.New("activity error"))
	// Display should still be called even if activity tracking fails
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingService).Once()

	orchestrator := application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		trackActivityUC,
		updateDisplayUC,
		true, // activity tracking enabled
	)

	// Act
	err := orchestrator.ExecuteRegularCheck()

	// Assert
	require.NoError(t, err) // Orchestrator doesn't fail on activity tracking error
	mockUIPort.AssertExpectations(t)
}
