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

func TestDetectClosedPRs_NoTrackedPRs_NoOp(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	currentPRs := testutil.CreateTestPRs(2, 0)

	// Act — no TrackPRs called, so trackedPRs is empty
	err := uc.Execute(context.Background(), currentPRs)

	// Assert
	require.NoError(t, err)
	// No calls to FetchPRStatus or Publish expected
}

func TestDetectClosedPRs_AllPRsStillOpen_NoEvents(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	prs := testutil.CreateTestPRs(3, 0)

	// Track PRs in first cycle
	uc.TrackPRs(prs)

	// Act — same PRs still present
	err := uc.Execute(context.Background(), prs)

	// Assert
	require.NoError(t, err)
	// No calls to FetchPRStatus since no PRs are missing
}

func TestDetectClosedPRs_MergedPR_EmitsMergedEvent(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	// Track both PRs
	uc.TrackPRs([]*pullrequest.PullRequest{pr1, pr2})

	// PR 1 disappears from the open list (merged)
	currentPRs := []*pullrequest.PullRequest{pr2}

	// Mock: GitHub says PR 1 is merged
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil)
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()

	// Act
	err := uc.Execute(context.Background(), currentPRs)

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}

func TestDetectClosedPRs_ClosedPR_EmitsClosedEvent(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	// Track both PRs
	uc.TrackPRs([]*pullrequest.PullRequest{pr1, pr2})

	// PR 2 disappears from the open list (closed)
	currentPRs := []*pullrequest.PullRequest{pr1}

	// Mock: GitHub says PR 2 is closed
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 2).Return(pullrequest.StatusClosed, nil)
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Closed)
		return ok
	})).Return(nil).Once()

	// Act
	err := uc.Execute(context.Background(), currentPRs)

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}

func TestDetectClosedPRs_APIError_KeepsPRTracked(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))

	// Track PR
	uc.TrackPRs([]*pullrequest.PullRequest{pr1})

	// PR disappears from open list
	currentPRs := []*pullrequest.PullRequest{}

	// Mock: API returns error on first call, then merged on second
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusOpen, errors.New("API error")).Once()
	// No Publish or UnmarkAsSeen calls expected on first Execute

	// Act
	err := uc.Execute(context.Background(), currentPRs)

	// Assert — no error returned (individual failures are logged, not propagated)
	require.NoError(t, err)

	// Verify PR is still tracked by running Execute again with the same empty list
	// This time GitHub says it's merged — if it was cleaned up, no FetchPRStatus would be called
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()

	err = uc.Execute(context.Background(), currentPRs)
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}

func TestDetectClosedPRs_PRStillOpenButMissing_KeepsTracked(t *testing.T) {
	// Arrange — PR disappears from search results but is still open (e.g., search index lag)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))

	// Track PR
	uc.TrackPRs([]*pullrequest.PullRequest{pr1})

	// PR disappears from open list
	currentPRs := []*pullrequest.PullRequest{}

	// Mock: GitHub says PR is still open
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusOpen, nil)
	// No Publish or UnmarkAsSeen calls expected

	// Act
	err := uc.Execute(context.Background(), currentPRs)

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	// No events published, PR kept tracked
}

func TestDetectClosedPRs_MultiplePRs_MixedStatuses(t *testing.T) {
	// Arrange — multiple PRs disappear with different final statuses
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))
	pr3 := testutil.NewTestPullRequest(3, testutil.WithURL("https://github.com/owner/repo/pull/3"))

	// Track all PRs
	uc.TrackPRs([]*pullrequest.PullRequest{pr1, pr2, pr3})

	// All PRs disappear from open list
	currentPRs := []*pullrequest.PullRequest{}

	// Mock: PR1 merged, PR2 closed, PR3 still open
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil)
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 2).Return(pullrequest.StatusClosed, nil)
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 3).Return(pullrequest.StatusOpen, nil)

	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Closed)
		return ok
	})).Return(nil).Once()

	// Act
	err := uc.Execute(context.Background(), currentPRs)

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}

func TestDetectClosedPRs_CleanupRemovesPRFromTracking(t *testing.T) {
	// Arrange — verify that after a PR is detected as merged, it's no longer tracked
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))

	// Track PR
	uc.TrackPRs([]*pullrequest.PullRequest{pr1})

	// First cycle: PR disappears, detected as merged
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)

	// Second cycle: same empty list, but no FetchPRStatus call since PR was cleaned up
	err = uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)

	// Assert — FetchPRStatus only called once (first cycle)
	mockPRRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_TrackPRs_UpdatesExistingEntry(t *testing.T) {
	// Arrange — TrackPRs should update the PR reference if called again with the same URL
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithTitle("Original"))
	pr1Updated := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithTitle("Updated"))

	// Track original, then updated version
	uc.TrackPRs([]*pullrequest.PullRequest{pr1})
	uc.TrackPRs([]*pullrequest.PullRequest{pr1Updated})

	// PR disappears
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		merged, ok := e.(*pullrequest.Merged)
		if !ok {
			return false
		}
		// The event should carry the updated PR reference
		return assert.Equal(t, "Updated", merged.PullRequest.Title())
	})).Return(nil).Once()

	// Act
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}
