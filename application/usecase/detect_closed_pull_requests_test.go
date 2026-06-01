package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/mocks"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDetectClosedPRs_NoTrackedPRs_NoOp(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)
	currentPRs := testutil.CreateTestPRs(2, 0)

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()

	urls, err := uc.Execute(context.Background(), currentPRs)

	require.NoError(t, err)
	assert.Empty(t, urls)
	mockPRRepo.AssertNotCalled(t, "FetchPRStatus")
	mockEventPublisher.AssertNotCalled(t, "Publish")
}

func TestDetectClosedPRs_AllPRsStillOpen_NoEvents(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)
	prs := testutil.CreateTestPRs(3, 0)

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	mockTrackingRepo.On("LoadAll").Return(prs, nil).Once()

	uc.TrackPRs(prs)
	urls, err := uc.Execute(context.Background(), prs)

	require.NoError(t, err)
	assert.Empty(t, urls)
	mockPRRepo.AssertNotCalled(t, "FetchPRStatus")
	mockEventPublisher.AssertNotCalled(t, "Publish")
}

func TestDetectClosedPRs_MergedPR_EmitsMergedEvent(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1, pr2}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	urls, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr2})

	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, pr1.URL(), urls[0])
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_ClosedPR_EmitsClosedEvent(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1, pr2}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 2).Return(pullrequest.StatusClosed, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Closed)
		return ok
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	urls, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr1})

	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, pr2.URL(), urls[0])
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_APIError_KeepsPRTracked(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusOpen, errors.New("API error")).Once()

	urls, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)
	assert.Empty(t, urls)

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	urls, err = uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, pr1.URL(), urls[0])
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_PRStillOpenButMissing_KeepsTracked(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)
	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusOpen, nil).Once()

	urls, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})

	require.NoError(t, err)
	assert.Empty(t, urls)
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
	mockEventPublisher.AssertNotCalled(t, "Publish")
}

func TestDetectClosedPRs_MultiplePRs_MixedStatuses(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))
	pr3 := testutil.NewTestPullRequest(3, testutil.WithURL("https://github.com/owner/repo/pull/3"))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1, pr2, pr3}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 2).Return(pullrequest.StatusClosed, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 3).Return(pullrequest.StatusOpen, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Closed)
		return ok
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	urls, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})

	require.NoError(t, err)
	require.Len(t, urls, 2)
	urlSet := make(map[string]bool)
	for _, url := range urls {
		urlSet[url] = true
	}
	assert.True(t, urlSet[pr1.URL()], "Expected pr1 URL in closed/merged list")
	assert.True(t, urlSet[pr2.URL()], "Expected pr2 URL in closed/merged list")
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_CleanupRemovesPRFromTracking(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)
	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		_, ok := e.(*pullrequest.Merged)
		return ok
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	urls, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, pr1.URL(), urls[0])

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	urls, err = uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)
	assert.Empty(t, urls)

	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_TrackPRs_UpdatesExistingEntry(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithTitle("Original"))
	pr1Updated := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithTitle("Updated"))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc.TrackPRs([]*pullrequest.PullRequest{pr1})
	uc.TrackPRs([]*pullrequest.PullRequest{pr1Updated})

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{pr1Updated}, nil).Once()
	mockPRRepo.On("FetchPRStatus", "owner", "repo", 1).Return(pullrequest.StatusMerged, nil).Once()
	mockEventPublisher.On("Publish", mock.MatchedBy(func(e pullrequest.Event) bool {
		merged, ok := e.(*pullrequest.Merged)
		if !ok {
			return false
		}
		return assert.Equal(t, "Updated", merged.PullRequest.Title())
	})).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	urls, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{})

	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, pr1Updated.URL(), urls[0])
	mockPRRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_TrackPRs_PreservesEnrichmentData(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)
	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))

	prevPR := testutil.NewTestPullRequest(1, testutil.WithURL(pr1.URL()))
	prevPR.SetHeadCommitSHA("abc123")
	prevPR.SetPipelineStatus(pullrequest.PipelineStatusSuccess)
	prevPR.SetLastActivityCheck(time.Now().Add(-1 * time.Hour))

	var savedPRs []*pullrequest.PullRequest
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{prevPR}, nil).Once()
	mockTrackingRepo.On("Save", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		savedPRs = prs
		return true
	})).Return(nil).Once()

	uc.TrackPRs([]*pullrequest.PullRequest{pr1})

	require.Len(t, savedPRs, 1)
	assert.Equal(t, "abc123", savedPRs[0].HeadCommitSHA())
	assert.Equal(t, pullrequest.PipelineStatusSuccess, savedPRs[0].PipelineStatus())
	assert.False(t, savedPRs[0].LastActivityCheck().IsZero())
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_TrackPRs_DoesNotMutateLivePRs(t *testing.T) {
	// Regression: TrackPRs used to mutate the live PR objects by calling
	// SetHeadCommitSHA/SetPipelineStatus/SetLastActivityCheck on them. Those
	// same objects are used downstream by TrackActivityUseCase and the display.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	livePR := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	// livePR has no enrichment data (as would come from a fresh API fetch).

	prevPR := testutil.NewTestPullRequest(1, testutil.WithURL(livePR.URL()))
	prevPR.SetHeadCommitSHA("old-sha")
	prevPR.SetPipelineStatus(pullrequest.PipelineStatusFailed)
	prevPR.SetLastActivityCheck(time.Now().Add(-2 * time.Hour))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{prevPR}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	originalSHA := livePR.HeadCommitSHA()
	originalPipeline := livePR.PipelineStatus()
	originalLastCheck := livePR.LastActivityCheck()

	uc.TrackPRs([]*pullrequest.PullRequest{livePR})

	// The live PR must be untouched — only the snapshot copy is modified.
	assert.Equal(t, originalSHA, livePR.HeadCommitSHA(), "TrackPRs must not mutate live PR headCommitSHA")
	assert.Equal(t, originalPipeline, livePR.PipelineStatus(), "TrackPRs must not mutate live PR pipelineStatus")
	assert.Equal(t, originalLastCheck, livePR.LastActivityCheck(), "TrackPRs must not mutate live PR lastActivityCheck")
}
