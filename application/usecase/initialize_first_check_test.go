package usecase_test

import (
	"context"
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
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewDraftFilter(false) // exclude drafts

	requestedPRs := testutil.CreateTestPRs(2, 1) // 2 regular, 1 draft
	userPRs := testutil.CreateTestPRs(1, 1)      // 1 regular, 1 draft

	// Mock expectations
	mockPRTrackingRepo.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(userPRs, nil)

	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockPRTrackingRepo).Once()
	mockPRTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockPRTrackingRepo, prFilter, mockUIPort)
	// Act
	isFirstRun, seededPRs, err := uc.Execute(context.Background())
	// Assert
	require.NoError(t, err)
	assert.True(t, isFirstRun, "Should return true on first run")
	assert.Len(t, seededPRs, 3, "Should return all 3 non-draft PRs")
	for _, pr := range seededPRs {
		assert.True(t, pr.Seen(), "all first-run PRs should be marked seen")
	}
	mockPRTrackingRepo.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}

func TestInitializeFirstCheck_NotFirstRun(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewDraftFilter(false)

	// Mock expectations
	mockPRTrackingRepo.On("IsEmpty").Return(false)
	// No other calls should be made

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockPRTrackingRepo, prFilter, mockUIPort)

	// Act
	isFirstRun, seededPRs, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err)
	assert.False(t, isFirstRun, "Should return false when not first run")
	assert.Nil(t, seededPRs, "Should return nil PRs when not first run")
	mockPRTrackingRepo.AssertExpectations(t)
	mockPRRepo.AssertNotCalled(t, "FetchRequestedReviews")
	mockPRRepo.AssertNotCalled(t, "FetchUserCreated")
	mockUIPort.AssertNotCalled(t, "UpdateDisplay")
}

func TestInitializeFirstCheck_FetchRequestedReviewsError(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewDraftFilter(false)

	expectedErr := errors.New("github api error")

	// Mock expectations
	mockPRTrackingRepo.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(nil, expectedErr)

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockPRTrackingRepo, prFilter, mockUIPort)

	// Act
	isFirstRun, seededPRs, err := uc.Execute(context.Background())

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.False(t, isFirstRun, "Should return false on error")
	assert.Nil(t, seededPRs)
	mockPRRepo.AssertNotCalled(t, "FetchUserCreated")
	mockUIPort.AssertNotCalled(t, "UpdateDisplay")
}

func TestInitializeFirstCheck_FetchUserCreatedError(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewDraftFilter(false)

	requestedPRs := testutil.CreateTestPRs(2, 0)
	expectedErr := errors.New("github api error")

	// Mock expectations
	mockPRTrackingRepo.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(nil, expectedErr)

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockPRTrackingRepo, prFilter, mockUIPort)

	// Act
	isFirstRun, seededPRs, err := uc.Execute(context.Background())

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.False(t, isFirstRun, "Should return false on error")
	assert.Nil(t, seededPRs)
	for _, pr := range requestedPRs {
		assert.False(t, pr.Seen())
	}
	mockUIPort.AssertNotCalled(t, "UpdateDisplay")
}

func TestInitializeFirstCheck_IncludeDrafts(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewDraftFilter(true) // include drafts
	mockPRTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	requestedPRs := testutil.CreateTestPRs(2, 1) // 2 regular, 1 draft
	userPRs := testutil.CreateTestPRs(1, 1)      // 1 regular, 1 draft

	// Mock expectations
	mockPRTrackingRepo.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(userPRs, nil)

	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockPRTrackingRepo).Once()

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockPRTrackingRepo, prFilter, mockUIPort)

	// Act
	isFirstRun, seededPRs, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err)
	assert.True(t, isFirstRun)
	assert.Len(t, seededPRs, 5, "Should return all 5 PRs including drafts")
	for _, pr := range seededPRs {
		assert.True(t, pr.Seen(), "all first-run PRs should be marked seen")
	}
	mockPRTrackingRepo.AssertExpectations(t)
}

func TestInitializeFirstCheck_NoPRs(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockPRTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockUIPort := mocks.NewUIPort(t)
	prFilter := pullrequest.NewDraftFilter(false)

	emptyPRs := []*pullrequest.PullRequest{} // empty slice, not nil

	// Mock expectations
	mockPRTrackingRepo.On("IsEmpty").Return(true)
	mockPRRepo.On("FetchRequestedReviews").Return(emptyPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(emptyPRs, nil)
	mockPRTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockPRTrackingRepo).Once()

	uc := usecase.NewInitializeFirstCheckUseCase(mockPRRepo, mockPRTrackingRepo, prFilter, mockUIPort)

	// Act
	isFirstRun, seededPRs, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err)
	assert.True(t, isFirstRun)
	assert.Empty(t, seededPRs, "Should return empty slice when no PRs exist")
	mockPRTrackingRepo.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockUIPort.AssertExpectations(t)
}
