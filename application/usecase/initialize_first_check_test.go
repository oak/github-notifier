package usecase_test

import (
	"errors"
	"testing"

	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/mocks"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInitializeFirstCheck_FirstRunEver(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewPRFilter(false) // exclude drafts

	requestedPRs := testutil.CreateTestPRs(2, 1) // 2 regular, 1 draft
	userPRs := testutil.CreateTestPRs(1, 1)      // 1 regular, 1 draft

	// Mock expectations
	mockTrackingService.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(userPRs, nil)

	// After filtering, should only mark non-drafts as seen (2 + 1 = 3)
	mockTrackingService.On("MarkPullRequestsAsSeen", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		return len(prs) == 2 && !prs[0].IsDraft() && !prs[1].IsDraft()
	})).Once()
	mockTrackingService.On("MarkPullRequestsAsSeen", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		return len(prs) == 1 && !prs[0].IsDraft()
	})).Once()

	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingService).Once()

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)

	// Act
	isFirstRun, err := uc.Execute()

	// Assert
	require.NoError(t, err)
	assert.True(t, isFirstRun, "Should return true on first run")
	mockTrackingService.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}

func TestInitializeFirstCheck_NotFirstRun(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewPRFilter(false)

	// Mock expectations
	mockTrackingService.On("IsEmpty").Return(false)
	// No other calls should be made

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)

	// Act
	isFirstRun, err := uc.Execute()

	// Assert
	require.NoError(t, err)
	assert.False(t, isFirstRun, "Should return false when not first run")
	mockTrackingService.AssertExpectations(t)
	mockPRRepo.AssertNotCalled(t, "FetchRequestedReviews")
	mockPRRepo.AssertNotCalled(t, "FetchUserCreated")
	mockUIPort.AssertNotCalled(t, "UpdateDisplay")
}

func TestInitializeFirstCheck_FetchRequestedReviewsError(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewPRFilter(false)

	expectedErr := errors.New("github api error")

	// Mock expectations
	mockTrackingService.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(nil, expectedErr)

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)

	// Act
	isFirstRun, err := uc.Execute()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.False(t, isFirstRun, "Should return false on error")
	mockPRRepo.AssertNotCalled(t, "FetchUserCreated")
	mockTrackingService.AssertNotCalled(t, "MarkPullRequestsAsSeen")
	mockUIPort.AssertNotCalled(t, "UpdateDisplay")
}

func TestInitializeFirstCheck_FetchUserCreatedError(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewPRFilter(false)

	requestedPRs := testutil.CreateTestPRs(2, 0)
	expectedErr := errors.New("github api error")

	// Mock expectations
	mockTrackingService.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(nil, expectedErr)

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)

	// Act
	isFirstRun, err := uc.Execute()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.False(t, isFirstRun, "Should return false on error")
	mockTrackingService.AssertNotCalled(t, "MarkPullRequestsAsSeen")
	mockUIPort.AssertNotCalled(t, "UpdateDisplay")
}

func TestInitializeFirstCheck_IncludeDrafts(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewPRFilter(true) // include drafts

	requestedPRs := testutil.CreateTestPRs(2, 1) // 2 regular, 1 draft
	userPRs := testutil.CreateTestPRs(1, 1)      // 1 regular, 1 draft

	// Mock expectations
	mockTrackingService.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(userPRs, nil)

	// When including drafts, all PRs should be marked as seen
	mockTrackingService.On("MarkPullRequestsAsSeen", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		return len(prs) == 3 // All requested PRs including draft
	})).Once()
	mockTrackingService.On("MarkPullRequestsAsSeen", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		return len(prs) == 2 // All user PRs including draft
	})).Once()

	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingService).Once()

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)

	// Act
	isFirstRun, err := uc.Execute()

	// Assert
	require.NoError(t, err)
	assert.True(t, isFirstRun)
	mockTrackingService.AssertExpectations(t)
}

func TestInitializeFirstCheck_NoPRs(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingService := mocks.NewService(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewPRFilter(false)

	emptyPRs := []*pullrequest.PullRequest{} // empty slice, not nil

	// Mock expectations
	mockTrackingService.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(emptyPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(emptyPRs, nil)
	mockTrackingService.On("MarkPullRequestsAsSeen", mock.AnythingOfType("[]*pullrequest.PullRequest")).Twice()
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingService).Once()

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockTrackingService, prFilter, mockUIPort)

	// Act
	isFirstRun, err := uc.Execute()

	// Assert
	require.NoError(t, err)
	assert.True(t, isFirstRun)
	mockTrackingService.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}
