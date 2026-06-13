package usecase_test

import (
	"context"
	"errors"
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

func TestDetectClosedPRs_NoTrackedPRs_NoOp(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)
	currentPRs := testutil.CreateTestPRs(2, 0)

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

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

	mockTrackingRepo.On("LoadAll").Return(prs, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

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
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

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

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

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
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	urls, err = uc.Execute(context.Background(), []*pullrequest.PullRequest{})
	require.NoError(t, err)
	assert.Empty(t, urls)

	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_DetectsUsingLatestSavedPRData(t *testing.T) {
	// Verifies that the snapshot saved by Execute is used in the next cycle's
	// detection — specifically that the event reflects the latest PR state.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	uc := usecase.NewDetectClosedPullRequestsUseCase(mockPRRepo, mockTrackingRepo, mockEventPublisher)

	pr1Updated := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithTitle("Updated"))

	// Detection cycle: pr1Updated has disappeared — event must carry the updated title.
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

func TestDetectClosedPRs_ExecutePreservesEnrichmentData(t *testing.T) {
	// Verifies that Execute preserves enrichment fields from the previous
	// snapshot when saving the current cycle's snapshot.
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

	_, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr1})

	require.NoError(t, err)
	require.Len(t, savedPRs, 1)
	assert.Equal(t, "abc123", savedPRs[0].HeadCommitSHA())
	assert.Equal(t, pullrequest.PipelineStatusSuccess, savedPRs[0].PipelineStatus())
	assert.False(t, savedPRs[0].LastActivityCheck().IsZero())
	mockTrackingRepo.AssertExpectations(t)
}

func TestDetectClosedPRs_ExecuteDoesNotMutateLivePRs(t *testing.T) {
	// Regression: saveSnapshot must not mutate the live PR objects — they are
	// used downstream by TrackActivityUseCase and the display after Execute.
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

	_, err := uc.Execute(context.Background(), []*pullrequest.PullRequest{livePR})
	require.NoError(t, err)

	// The live PR must be untouched — only the snapshot copy is modified.
	assert.Equal(t, originalSHA, livePR.HeadCommitSHA(), "Execute must not mutate live PR headCommitSHA")
	assert.Equal(t, originalPipeline, livePR.PipelineStatus(), "Execute must not mutate live PR pipelineStatus")
	assert.Equal(t, originalLastCheck, livePR.LastActivityCheck(), "Execute must not mutate live PR lastActivityCheck")
}
