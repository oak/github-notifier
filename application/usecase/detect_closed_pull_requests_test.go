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
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	currentPRs := testutil.CreateTestPRs(2, 0)

	// Execute loads from the repo — returns empty (nothing tracked yet)
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	// Act — no TrackPRs called, so nothing is in the repo
	err := uc.Execute(context.Background(), currentPRs)

	// Assert
	require.NoError(t, err)
	// No calls to FetchPRStatus or Publish expected
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_AllPRsStillOpen_NoEvents(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	prs := testutil.CreateTestPRs(3, 0)
	snapshots := make([]pullrequest.PRStateSnapshot, len(prs))
	for i, pr := range prs {
		snapshots[i] = pr.ToSnapshot()
	}

	// TrackPRs: load existing (empty), save new set
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// Execute: load the saved set — all PRs still present
	mockTrackingRepo.On("LoadAll").Return(snapshots, nil).Once()

	// Track PRs then execute with same PRs still present
	uc.TrackPRs(prs)
	err := uc.Execute(context.Background(), prs)

	// Assert
	require.NoError(t, err)
	// No calls to FetchPRStatus since no PRs are missing
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_MergedPR_EmitsMergedEvent(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	snap1 := pr1.ToSnapshot()
	snap2 := pr2.ToSnapshot()

	// Execute: LoadAll returns both snapshots; pr1 is missing from current list
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{snap1, snap2}, nil).Once()

	// Mock: GitHub says PR 1 is merged
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil)
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()

	// After cleanup: Save with only snap2 remaining
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// Act — only pr2 in current list
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr2})

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_ClosedPR_EmitsClosedEvent(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	snap1 := pr1.ToSnapshot()
	snap2 := pr2.ToSnapshot()

	// Execute: LoadAll returns both; pr2 missing
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{snap1, snap2}, nil).Once()

	// Mock: GitHub says PR 2 is closed
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 2).Return(pullrequest.StatusClosed, nil)
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Closed)
		return ok
	})).Return(nil).Once()

	// After cleanup: Save with only snap1 remaining
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// Act — only pr1 in current list
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr1})

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_APIError_KeepsPRTracked(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	snap1 := pr1.ToSnapshot()

	// First Execute: LoadAll returns snap1; API error — no Save (PR kept tracked)
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{snap1}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusOpen, errors.New("API error")).Once()

	// Act — first call
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)

	// Verify PR is still tracked: second Execute also gets LoadAll with snap1
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{snap1}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	err = uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_PRStillOpenButMissing_KeepsTracked(t *testing.T) {
	// Arrange — PR disappears from search results but is still open (e.g., search index lag)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	snap1 := pr1.ToSnapshot()

	// Execute: LoadAll returns snap1; GitHub says still open — no cleanup Save
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{snap1}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusOpen, nil)

	// Act
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
	// No events published, no cleanup Save
}

func TestDetectClosedPRs_MultiplePRs_MixedStatuses(t *testing.T) {
	// Arrange — multiple PRs disappear with different final statuses
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))
	pr3 := testutil.NewTestPullRequest(3, testutil.WithURL("https://github.com/owner/repo/pull/3"))

	snap1 := pr1.ToSnapshot()
	snap2 := pr2.ToSnapshot()
	snap3 := pr3.ToSnapshot()

	// Execute: all three tracked, all missing from current list
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{snap1, snap2, snap3}, nil).Once()

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

	// After cleanup: Save with only snap3 remaining (pr1+pr2 processed)
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// Act — all disappear
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_CleanupRemovesPRFromTracking(t *testing.T) {
	// Arrange — verify that after a PR is detected as merged, it's no longer tracked
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	snap1 := pr1.ToSnapshot()

	// First Execute: LoadAll returns snap1; merged → Save([]) cleanup
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{snap1}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)

	// Second Execute: LoadAll returns empty → early return, no FetchPRStatus
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	err = uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)

	// FetchPRStatus only called once (first cycle)
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_TrackPRs_UpdatesExistingEntry(t *testing.T) {
	// Arrange — TrackPRs called twice with same URL but different title; Execute
	// should use the most recently tracked version.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithTitle("Original"))
	pr1Updated := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithTitle("Updated"))

	// First TrackPRs: load empty, save [Original]
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// Second TrackPRs: load [Original], save [Updated]
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{pr1.ToSnapshot()}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc.TrackPRs([]*pullrequest.PullRequest{pr1})
	uc.TrackPRs([]*pullrequest.PullRequest{pr1Updated})

	// Execute: LoadAll returns the Updated snapshot
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{pr1Updated.ToSnapshot()}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		merged, ok := e.(*pullrequest.Merged)
		if !ok {
			return false
		}
		// The event should carry the updated PR reference
		return assert.Equal(t, "Updated", merged.PullRequest.Title())
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// Act
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_TrackPRs_PreservesEnrichmentData(t *testing.T) {
	// Arrange — TrackPRs should preserve enrichment fields (HeadCommitSHA,
	// PipelineStatus, LastActivityCheck) from the previously-saved snapshot.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))

	// Simulate a previous snapshot that has enrichment data
	prevSnap := pr1.ToSnapshot()
	prevSnap.HeadCommitSHA = "abc123"
	prevSnap.PipelineStatus = pullrequest.PipelineStatusSuccess

	// TrackPRs: load previous snapshot with enrichment, save merged result
	var savedSnapshots []pullrequest.PRStateSnapshot
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{prevSnap}, nil).Once()
	mockTrackingRepo.On("Save", mock.MatchedBy(func(snaps []pullrequest.PRStateSnapshot) bool {
		savedSnapshots = snaps
		return true
	})).Return(nil).Once()

	uc.TrackPRs([]*pullrequest.PullRequest{pr1})

	// Assert: saved snapshot preserves enrichment from previous
	require.Len(t, savedSnapshots, 1)
	assert.Equal(t, "abc123", savedSnapshots[0].HeadCommitSHA)
	assert.Equal(t, pullrequest.PipelineStatusSuccess, savedSnapshots[0].PipelineStatus)
	mockTrackingRepo.AssertExpectations(t)
}
