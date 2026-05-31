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

func TestCheckNewPRs_NewPRs_PublishesEventsAndMarksSeen(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewDraftFilter(false)

	requestedPRs := testutil.CreateTestPRs(2, 0)
	userPRs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(10, testutil.WithURL("https://github.com/owner/repo/pull/10")),
	}

	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(userPRs, nil)
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Times(3)

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, nil, prFilter, mockEventPublisher)

	result, _, err := uc.Execute(context.Background(), usecase.NewCheckCycleState())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.RequestedReviewPRs, 2)
	assert.Len(t, result.UserCreatedPRs, 1)
	for _, pr := range result.RequestedReviewPRs {
		assert.True(t, pr.Seen())
	}
	for _, pr := range result.UserCreatedPRs {
		assert.True(t, pr.Seen())
	}
	mockEventPublisher.AssertExpectations(t)
}

func TestCheckNewPRs_PRsWithActivity_AreSeenButNoNewPREvents(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewDraftFilter(false)

	state := usecase.NewCheckCycleState()
	time.Sleep(10 * time.Millisecond)

	now := time.Now()
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-1*time.Hour)))

	activity1 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now, testutil.WithActivityPR(pr1.URL(), pr1.Number()))
	activity2 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now, testutil.WithActivityPR(pr2.URL(), pr2.Number()))
	pr1.AddActivities([]*pullrequest.Activity{activity1})
	pr2.AddActivities([]*pullrequest.Activity{activity2})

	prsWithActivity := []*pullrequest.PullRequest{pr1, pr2}
	mockPRRepo.On("FetchRequestedReviews").Return(prsWithActivity, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, nil, prFilter, mockEventPublisher)

	result, _, err := uc.Execute(context.Background(), state)

	require.NoError(t, err)
	require.NotNil(t, result)
	mockEventPublisher.AssertNotCalled(t, "Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected"))
	for _, pr := range prsWithActivity {
		assert.True(t, pr.Seen(), "all newly observed PRs should be marked seen")
	}
}

func TestCheckNewPRs_MixedNewAndActivity_OnlyTrulyNewEmitEvents(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewDraftFilter(false)

	state := usecase.NewCheckCycleState()
	time.Sleep(10 * time.Millisecond)

	now := time.Now()
	newPR1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	newPR2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"), testutil.WithCreatedAt(now.Add(-10*time.Minute)))

	activePR1 := testutil.NewTestPullRequest(3, testutil.WithURL("https://github.com/owner/repo/pull/3"), testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	activePR2 := testutil.NewTestPullRequest(4, testutil.WithURL("https://github.com/owner/repo/pull/4"), testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	activity1 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now, testutil.WithActivityPR(activePR1.URL(), activePR1.Number()))
	activity2 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now, testutil.WithActivityPR(activePR2.URL(), activePR2.Number()))
	activePR1.AddActivities([]*pullrequest.Activity{activity1})
	activePR2.AddActivities([]*pullrequest.Activity{activity2})

	allPRs := []*pullrequest.PullRequest{newPR1, newPR2, activePR1, activePR2}
	mockPRRepo.On("FetchRequestedReviews").Return(allPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Twice()

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, nil, prFilter, mockEventPublisher)

	result, _, err := uc.Execute(context.Background(), state)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.RequestedReviewPRs, 4)
	for _, pr := range allPRs {
		assert.True(t, pr.Seen())
	}
	mockEventPublisher.AssertExpectations(t)
}

func TestCheckNewPRs_FetchRequestedReviewsError(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewDraftFilter(false)

	expectedErr := errors.New("github api error")
	mockPRRepo.On("FetchRequestedReviews").Return(nil, expectedErr)

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, nil, prFilter, mockEventPublisher)

	result, _, err := uc.Execute(context.Background(), usecase.NewCheckCycleState())

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
	mockPRRepo.AssertNotCalled(t, "FetchUserCreated")
}

func TestCheckNewPRs_FetchUserCreatedError(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewDraftFilter(false)

	requestedPRs := testutil.CreateTestPRs(2, 0)
	expectedErr := errors.New("github api error")

	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(nil, expectedErr)

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, nil, prFilter, mockEventPublisher)

	result, _, err := uc.Execute(context.Background(), usecase.NewCheckCycleState())

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
}

func TestCheckNewPRs_FiltersDrafts(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewDraftFilter(false)

	requestedPRs := testutil.CreateTestPRs(2, 2)

	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Twice()

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, nil, prFilter, mockEventPublisher)

	result, _, err := uc.Execute(context.Background(), usecase.NewCheckCycleState())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.RequestedReviewPRs, 2)
	testutil.AssertNoDrafts(t, result.RequestedReviewPRs)
	for _, pr := range result.RequestedReviewPRs {
		assert.True(t, pr.Seen())
	}
}

func TestCheckNewPRs_PublishEventError_ContinuesProcessing(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewDraftFilter(false)

	newPRs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1")),
		testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2")),
	}

	mockPRRepo.On("FetchRequestedReviews").Return(newPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(errors.New("event bus error")).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Once()

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, nil, prFilter, mockEventPublisher)

	result, _, err := uc.Execute(context.Background(), usecase.NewCheckCycleState())

	require.NoError(t, err)
	require.NotNil(t, result)
	for _, pr := range newPRs {
		assert.True(t, pr.Seen(), "PRs should be marked seen even if publish fails")
	}
	mockEventPublisher.AssertExpectations(t)
}

func TestCheckNewPRs_SeedsKnownReviewsFromSnapshotsOnFirstCycle(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewDraftFilter(false)

	now := time.Now()
	pr := testutil.NewTestPullRequest(
		42,
		testutil.WithURL("https://github.com/owner/repo/pull/42"),
		testutil.WithTitle("Review baseline test"),
		testutil.WithCreatedAt(now.Add(-24*time.Hour)),
	)

	reviewer := testutil.NewTestAuthor("reviewer")
	pr.AddReview(pullrequest.NewReview(reviewer, pullrequest.ReviewStateApproved, now.Add(-1*time.Hour)))

	seedPR := testutil.NewTestPullRequest(
		42,
		testutil.WithURL(pr.URL()),
		testutil.WithTitle(pr.Title()),
		testutil.WithRepository(pr.RepositoryName()),
		testutil.WithAuthor(pr.AuthorLogin()),
		testutil.WithCreatedAt(pr.CreatedAt()),
	)
	seedPR.SetInitialLastActivityCheck(now.Add(-2 * time.Hour))
	seedPR.SetInitialPipelineStatus(pullrequest.PipelineStatusUnknown)
	seedPR.SetInitialReviews(map[string]*pullrequest.Review{
		"reviewer": pullrequest.NewReview(testutil.NewTestAuthor("reviewer"), pullrequest.ReviewStateCommented, now.Add(-2*time.Hour)),
	})

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{seedPR}, nil).Once()
	mockPRRepo.On("FetchRequestedReviews").Return([]*pullrequest.PullRequest{pr}, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ReviewStateChanged")).Return(nil).Once()

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, mockTrackingRepo, prFilter, mockEventPublisher)
	state := usecase.NewCheckCycleState()
	// Keep PR as known so this test isolates review-change seeding behavior and
	// does not require NewPullRequestDetected expectations.
	state.KnownPRs[pr.URL()] = true

	_, state, err := uc.Execute(context.Background(), state)

	require.NoError(t, err)
	assert.True(t, state.ReviewsSeeded)
	known, ok := state.KnownReviews[pr.URL()]
	require.True(t, ok)
	require.Contains(t, known, "reviewer")
	assert.Equal(t, pullrequest.ReviewStateApproved, known["reviewer"].State())
}
