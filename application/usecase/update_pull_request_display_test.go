package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/oak/github-notifier/application/usecase"
	"github.com/oak/github-notifier/domain/pullrequest"
	"github.com/oak/github-notifier/internal/mocks"
	"github.com/oak/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUpdateDisplay_SortsByCreatedDate(t *testing.T) {
	// Arrange
	mockUIPort := mocks.NewUIPort(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)

	now := time.Now()

	// Create PRs in random order (not sorted by date)
	pr3 := testutil.NewTestPullRequest(3, testutil.WithCreatedAt(now.Add(-30*time.Minute)))
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-2*time.Hour)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-1*time.Hour)))

	requestedPRs := []*pullrequest.PullRequest{pr3, pr1, pr2}
	userPRs := []*pullrequest.PullRequest{}

	// Mock expectations - verify PRs are sorted (oldest first)
	mockUIPort.On("UpdateDisplay",
		mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
			// Should be sorted: pr1 (2h ago), pr2 (1h ago), pr3 (30m ago)
			return len(prs) == 3 &&
				prs[0].Number() == 1 &&
				prs[1].Number() == 2 &&
				prs[2].Number() == 3
		}),
		userPRs,
		mockTrackingRepo,
	).Return()

	uc := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingRepo)

	// Act
	err := uc.Execute(context.Background(), requestedPRs, userPRs)

	// Assert
	require.NoError(t, err)
	mockUIPort.AssertExpectations(t)
}

func TestUpdateDisplay_EmptyPRs(t *testing.T) {
	// Arrange
	mockUIPort := mocks.NewUIPort(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)

	var emptyPRs []*pullrequest.PullRequest

	// sortedByCreatedAt always returns a new (non-nil) empty slice
	mockUIPort.On("UpdateDisplay",
		mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool { return len(prs) == 0 }),
		mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool { return len(prs) == 0 }),
		mockTrackingRepo,
	).Return()

	uc := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingRepo)

	// Act
	err := uc.Execute(context.Background(), emptyPRs, emptyPRs)

	// Assert
	require.NoError(t, err)
	mockUIPort.AssertExpectations(t)
}

func TestUpdateDisplay_BothRequestedAndUserPRs(t *testing.T) {
	// Arrange
	mockUIPort := mocks.NewUIPort(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)

	now := time.Now()

	// Create requested review PRs (unsorted)
	reqPR2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	reqPR1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-2*time.Hour)))
	requestedPRs := []*pullrequest.PullRequest{reqPR2, reqPR1}

	// Create user PRs (unsorted)
	userPR2 := testutil.NewTestPullRequest(4, testutil.WithCreatedAt(now.Add(-30*time.Minute)))
	userPR1 := testutil.NewTestPullRequest(3, testutil.WithCreatedAt(now.Add(-45*time.Minute)))
	userPRs := []*pullrequest.PullRequest{userPR2, userPR1}

	// Mock expectations - both lists should be sorted independently
	mockUIPort.On("UpdateDisplay",
		mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
			// Requested PRs sorted
			return len(prs) == 2 && prs[0].Number() == 1 && prs[1].Number() == 2
		}),
		mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
			// User PRs sorted
			return len(prs) == 2 && prs[0].Number() == 3 && prs[1].Number() == 4
		}),
		mockTrackingRepo,
	).Return()

	uc := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingRepo)

	// Act
	err := uc.Execute(context.Background(), requestedPRs, userPRs)

	// Assert
	require.NoError(t, err)
	mockUIPort.AssertExpectations(t)
}

func TestUpdateDisplay_SinglePR(t *testing.T) {
	// Arrange
	mockUIPort := mocks.NewUIPort(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)

	pr := testutil.NewTestPullRequest(1)
	prs := []*pullrequest.PullRequest{pr}

	mockUIPort.On("UpdateDisplay", prs, []*pullrequest.PullRequest{}, mockTrackingRepo).Return()

	uc := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingRepo)

	// Act
	err := uc.Execute(context.Background(), prs, []*pullrequest.PullRequest{})

	// Assert
	require.NoError(t, err)
	mockUIPort.AssertExpectations(t)
}

func TestUpdateDisplay_PreservesOriginalSlice(t *testing.T) {
	// Arrange
	mockUIPort := mocks.NewUIPort(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)

	now := time.Now()
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-2*time.Hour)))
	originalOrder := []*pullrequest.PullRequest{pr1, pr2}

	mockUIPort.On("UpdateDisplay", mock.AnythingOfType("[]*pullrequest.PullRequest"), mock.AnythingOfType("[]*pullrequest.PullRequest"), mockTrackingRepo).Return()

	uc := usecase.NewUpdatePullRequestDisplayUseCase(mockUIPort, mockTrackingRepo)

	// Act
	err := uc.Execute(context.Background(), originalOrder, []*pullrequest.PullRequest{})

	// Assert
	require.NoError(t, err)
	// Original slice must NOT be modified (sortedByCreatedAt returns a copy)
	assert.Equal(t, 1, originalOrder[0].Number(), "Original slice must preserve insertion order")
	assert.Equal(t, 2, originalOrder[1].Number())
}
