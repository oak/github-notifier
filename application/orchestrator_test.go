package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oak3/github-notifier/application"
	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/mocks"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func buildOrchestrator(
	t *testing.T,
	mockPRRepo *mocks.PullRequestRepository,
	mockTrackingRepo *mocks.PRTrackingRepository,
	mockUIPort *mocks.UIPort,
	mockEventPublisher *mocks.EventPublisher,
	enableActivityTracking bool,
) *application.PullRequestOrchestrator {
	prFilter := pullrequest.NewDraftFilter(false)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	initUC := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingRepo, prFilter, mockUIPort)
	checkNewPRsUC := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, mockTrackingRepo, prFilter, mockEventPublisher)
	detectClosedPRsUC := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)
	trackActivityUC := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")
	updateDisplayUC := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingRepo)

	return application.NewPullRequestOrchestrator(
		initUC,
		checkNewPRsUC,
		detectClosedPRsUC,
		trackActivityUC,
		updateDisplayUC,
		enableActivityTracking,
	)
}

func TestOrchestrator_ExecuteInitialCheck_FirstRun(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	prs := testutil.CreateTestPRs(2, 0)

	mockTrackingRepo.On("IsEmpty").Return(true).Once()
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil).Once()
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockUIPort.On("UpdateDisplay", mock.Anything, mock.Anything, mockTrackingRepo).Once()
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	orchestrator := buildOrchestrator(t, mockPRRepo, mockTrackingRepo, mockUIPort, mockEventPublisher, false)

	err := orchestrator.ExecuteInitialCheck(context.Background())

	require.NoError(t, err)
	for _, pr := range prs {
		assert.True(t, pr.Seen(), "first-run PRs should be marked seen")
	}
}

func TestOrchestrator_ExecuteInitialCheck_NotFirstRun(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	prs := testutil.CreateTestPRs(2, 0)

	mockTrackingRepo.On("IsEmpty").Return(false).Once()
	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil).Once()
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Twice()
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingRepo.On("Save", mock.Anything).Return(nil)
	mockUIPort.On("UpdateDisplay", mock.Anything, mock.Anything, mockTrackingRepo).Once()

	orchestrator := buildOrchestrator(t, mockPRRepo, mockTrackingRepo, mockUIPort, mockEventPublisher, false)

	err := orchestrator.ExecuteInitialCheck(context.Background())

	require.NoError(t, err)
}

func TestOrchestrator_ExecuteRegularCheck_WithoutActivityTracking(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	prs := testutil.CreateTestPRs(2, 0)

	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil).Once()
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Twice()
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingRepo.On("Save", mock.Anything).Return(nil)
	mockUIPort.On("UpdateDisplay", mock.Anything, mock.Anything, mockTrackingRepo).Once()

	orchestrator := buildOrchestrator(t, mockPRRepo, mockTrackingRepo, mockUIPort, mockEventPublisher, false)

	err := orchestrator.ExecuteRegularCheck(context.Background(), time.Now())

	require.NoError(t, err)
	mockPRRepo.AssertNotCalled(t, "FetchActivities")
}

func TestOrchestrator_ExecuteRegularCheck_WithActivityTracking(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	prs := testutil.CreateTestPRs(2, 0)

	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil).Once()
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Twice()
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingRepo.On("Save", mock.Anything).Return(nil)
	mockPRRepo.On("FetchActivities", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("time.Time")).Return(map[string]pullrequest.PRActivityData{}, nil).Once()
	mockUIPort.On("UpdateDisplay", mock.Anything, mock.Anything, mockTrackingRepo).Once()

	orchestrator := buildOrchestrator(t, mockPRRepo, mockTrackingRepo, mockUIPort, mockEventPublisher, true)

	err := orchestrator.ExecuteRegularCheck(context.Background(), time.Now())

	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
}

func TestOrchestrator_ExecuteRegularCheck_AllPRsClosed_UpdatesDisplayWithEmptyLists(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	tracked := testutil.NewTestPullRequest(1)

	mockPRRepo.On("FetchRequestedReviews").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{tracked}, nil).Times(3)
	mockPRRepo.On("FetchPRStatus", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).
		Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.Anything).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Twice()
	mockUIPort.On("UpdateDisplay",
		mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool { return len(prs) == 0 }),
		mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool { return len(prs) == 0 }),
		mockTrackingRepo,
	).Once()

	orchestrator := buildOrchestrator(t, mockPRRepo, mockTrackingRepo, mockUIPort, mockEventPublisher, false)

	err := orchestrator.ExecuteRegularCheck(context.Background(), time.Now())

	require.NoError(t, err)
	mockUIPort.AssertExpectations(t)
}

func TestOrchestrator_ExecuteRegularCheck_CheckNewPRsError(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	expectedErr := errors.New("github api error")
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchRequestedReviews").Return(nil, expectedErr).Once()

	orchestrator := buildOrchestrator(t, mockPRRepo, mockTrackingRepo, mockUIPort, mockEventPublisher, false)

	err := orchestrator.ExecuteRegularCheck(context.Background(), time.Now())

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockPRRepo.AssertNotCalled(t, "FetchActivities")
	mockUIPort.AssertNotCalled(t, "UpdateDisplay")
}

func TestOrchestrator_ExecuteRegularCheck_ActivityTrackingError_ContinuesWithDisplay(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	prs := testutil.CreateTestPRs(2, 0)

	mockPRRepo.On("FetchRequestedReviews").Return(prs, nil).Once()
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Twice()
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil)
	mockTrackingRepo.On("Save", mock.Anything).Return(nil)
	mockPRRepo.On("FetchActivities", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("time.Time")).Return(map[string]pullrequest.PRActivityData{}, errors.New("activity error")).Once()
	mockUIPort.On("UpdateDisplay", mock.Anything, mock.Anything, mockTrackingRepo).Once()

	orchestrator := buildOrchestrator(t, mockPRRepo, mockTrackingRepo, mockUIPort, mockEventPublisher, true)

	err := orchestrator.ExecuteRegularCheck(context.Background(), time.Now())

	require.NoError(t, err)
	mockUIPort.AssertExpectations(t)
}
