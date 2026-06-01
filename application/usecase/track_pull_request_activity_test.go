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

func TestTrackActivity_EmptyPRList(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{}, time.Now())

	require.NoError(t, err)
	mockPRRepo.AssertNotCalled(t, "FetchActivities")
	mockTrackingRepo.AssertNotCalled(t, "LoadAll")
}

func TestTrackActivity_NoPRsDueForCheck(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-96*time.Hour)))
	prs := []*pullrequest.PullRequest{pr1, pr2}
	scheduler.MarkCheckedAt(now, prs)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), prs, now.Add(-1*time.Hour))

	require.NoError(t, err)
	mockPRRepo.AssertNotCalled(t, "FetchActivities")
	mockTrackingRepo.AssertNotCalled(t, "LoadAll")
}

func TestTrackActivity_NoNewActivity(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	prs := testutil.CreateTestPRs(2, 0)
	for _, pr := range prs {
		pr.MarkAsSeen()
	}

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", prs, lastCheckTime).Return(map[string]pullrequest.PRActivityData{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), prs, lastCheckTime)

	require.NoError(t, err)
	for _, pr := range prs {
		assert.True(t, pr.Seen())
	}
	mockEventPublisher.AssertNotCalled(t, "Publish")
}

func TestTrackActivity_NewActivity_EmitsEventsAndMarksUnseen(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-15*time.Minute)))
	pr1.MarkAsSeen()
	pr2.MarkAsSeen()
	prs := []*pullrequest.PullRequest{pr1, pr2}

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", prs, lastCheckTime).Return(map[string]pullrequest.PRActivityData{
		pr1.URL(): {
			Activities: []*pullrequest.Activity{testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr1.URL(), pr1.Number()),
			)},
		},
		pr2.URL(): {
			Activities: []*pullrequest.Activity{testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr2.URL(), pr2.Number()),
			)},
		},
	}, nil).Once()

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Twice()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), prs, lastCheckTime)

	require.NoError(t, err)
	assert.False(t, pr1.Seen())
	assert.False(t, pr2.Seen())
	mockEventPublisher.AssertExpectations(t)
}

func TestTrackActivity_EnrichError(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	prs := testutil.CreateTestPRs(2, 0)

	expectedErr := errors.New("github api error")
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", prs, lastCheckTime).Return(map[string]pullrequest.PRActivityData{}, expectedErr).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), prs, lastCheckTime)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockTrackingRepo.AssertNotCalled(t, "Save")
	mockEventPublisher.AssertNotCalled(t, "Publish")
}

func TestTrackActivity_PublishEventError_ContinuesProcessing(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-15*time.Minute)))
	pr1.MarkAsSeen()
	pr2.MarkAsSeen()
	prs := []*pullrequest.PullRequest{pr1, pr2}

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", prs, lastCheckTime).Return(map[string]pullrequest.PRActivityData{
		pr1.URL(): {
			Activities: []*pullrequest.Activity{testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr1.URL(), pr1.Number()),
			)},
		},
		pr2.URL(): {
			Activities: []*pullrequest.Activity{testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr2.URL(), pr2.Number()),
			)},
		},
	}, nil).Once()

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(errors.New("event bus error")).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), prs, lastCheckTime)

	require.NoError(t, err)
	assert.False(t, pr1.Seen())
	assert.False(t, pr2.Seen())
	mockEventPublisher.AssertExpectations(t)
}

func TestTrackActivity_TwoTierScheduling(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	recentPR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-24*time.Hour)))
	stalePR := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	prs := []*pullrequest.PullRequest{recentPR, stalePR}

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", prs, lastCheckTime).Return(map[string]pullrequest.PRActivityData{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), prs, lastCheckTime)
	require.NoError(t, err)

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", mock.MatchedBy(func(prsToCheck []*pullrequest.PullRequest) bool {
		return len(prsToCheck) == 1 && prsToCheck[0].Number() == 1
	}), lastCheckTime).Return(map[string]pullrequest.PRActivityData{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	err = uc.Execute(context.Background(), prs, lastCheckTime)
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_LoadsAndSeedsStateFromRepository(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))

	prevPR := testutil.NewTestPullRequest(1, testutil.WithURL(pr1.URL()), testutil.WithCreatedAt(pr1.CreatedAt()))
	prevPR.SetHeadCommitSHA("abc123")
	prevPR.SetPipelineStatus(pullrequest.PipelineStatusSuccess)

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{prevPR}, nil).Once()

	var seededSHA string
	var seededStatus pullrequest.PipelineStatus
	mockPRRepo.On("FetchActivities", mock.MatchedBy(func(prsToCheck []*pullrequest.PullRequest) bool {
		seededSHA = prsToCheck[0].HeadCommitSHA()
		seededStatus = prsToCheck[0].PipelineStatus()
		return true
	}), lastCheckTime).Return(map[string]pullrequest.PRActivityData{}, nil).Once()

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr1}, lastCheckTime)

	require.NoError(t, err)
	assert.Equal(t, "abc123", seededSHA)
	assert.Equal(t, pullrequest.PipelineStatusSuccess, seededStatus)
}

func TestTrackActivity_PersistsLastActivityCheckAfterExecution(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", mock.Anything, lastCheckTime).Return(map[string]pullrequest.PRActivityData{}, nil).Once()

	var savedPRs []*pullrequest.PullRequest
	mockTrackingRepo.On("Save", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		savedPRs = prs
		return true
	})).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	beforeExec := time.Now()
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr1}, lastCheckTime)
	afterExec := time.Now()

	require.NoError(t, err)
	require.Len(t, savedPRs, 1)
	lac := savedPRs[0].LastActivityCheck()
	assert.False(t, lac.IsZero())
	assert.True(t, !lac.Before(beforeExec) && !lac.After(afterExec))
}

func TestTrackActivity_IgnoredAuthor_NotMarkedUnseen(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)), testutil.WithRepository("owner/repo"))
	pr.MarkAsSeen()

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", mock.Anything, lastCheckTime).Return(map[string]pullrequest.PRActivityData{
		pr.URL(): {
			Activities: []*pullrequest.Activity{testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr.URL(), pr.Number()),
				testutil.WithActivityAuthor("dependabot"),
			)},
		},
	}, nil).Once()

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "alice")
	ignoreCfg := &pullrequest.IgnoreConfig{}
	ignoreCfg.Ignore.Global.AuthoredBy = []pullrequest.IgnoreActorRule{{Login: "dependabot"}}
	uc.UpdateIgnoreConfig(ignoreCfg)

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr}, lastCheckTime)

	require.NoError(t, err)
	assert.True(t, pr.Seen())
}

func TestTrackActivity_IgnoreConfig_NonIgnoredAuthorStillMarkUnseen(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	pr := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)), testutil.WithRepository("owner/repo"))
	pr.MarkAsSeen()

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", mock.Anything, lastCheckTime).Return(map[string]pullrequest.PRActivityData{
		pr.URL(): {
			Activities: []*pullrequest.Activity{testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr.URL(), pr.Number()),
				testutil.WithActivityAuthor("bob"),
			)},
		},
	}, nil).Once()

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "alice")
	ignoreCfg := &pullrequest.IgnoreConfig{}
	ignoreCfg.Ignore.Global.AuthoredBy = []pullrequest.IgnoreActorRule{{Login: "dependabot"}}
	uc.UpdateIgnoreConfig(ignoreCfg)

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr}, lastCheckTime)

	require.NoError(t, err)
	assert.False(t, pr.Seen())
}

func TestTrackActivity_RestartScenario_StaleCheckedOnceNotForever(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	stalePR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("FetchActivities", mock.Anything, lastCheckTime).Return(map[string]pullrequest.PRActivityData{}, nil).Once()

	var savedAfterCycle1 []*pullrequest.PullRequest
	mockTrackingRepo.On("Save", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		savedAfterCycle1 = prs
		return true
	})).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{stalePR}, lastCheckTime)
	require.NoError(t, err)
	require.Len(t, savedAfterCycle1, 1)
	assert.False(t, savedAfterCycle1[0].LastActivityCheck().IsZero())

	err = uc.Execute(context.Background(), []*pullrequest.PullRequest{stalePR}, lastCheckTime)
	require.NoError(t, err)
	mockPRRepo.AssertNumberOfCalls(t, "FetchActivities", 1)
}

func TestTrackActivity_StalePRsPreservedInTrackingRepo(t *testing.T) {
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	// Stale threshold = 1h, check interval = 999min so stale PR is never re-checked.
	scheduler := pullrequest.NewActivityCheckScheduler(1, 999)

	now := time.Now()
	lastCheckTime := now.Add(-10 * time.Minute)

	recentPR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-30*time.Minute)))
	stalePR := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-2*time.Hour)))
	stalePR.SetHeadCommitSHA("stale-sha")
	stalePR.SetPipelineStatus(pullrequest.PipelineStatusSuccess)

	// Tell the scheduler the stale PR was checked very recently so it won't be
	// included in prsToCheck this cycle (interval = 999min).
	scheduler.SeedLastChecked(stalePR.URL(), now.Add(-1*time.Minute))

	// Tracking repo already holds the stale PR with enrichment state.
	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{stalePR}, nil)
	mockPRRepo.On("FetchActivities", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		// Only recentPR should be in prsToCheck — stalePR is skipped this cycle.
		return len(prs) == 1 && prs[0].URL() == recentPR.URL()
	}), lastCheckTime).Return(map[string]pullrequest.PRActivityData{}, nil)

	var saved []*pullrequest.PullRequest
	mockTrackingRepo.On("Save", mock.MatchedBy(func(prs []*pullrequest.PullRequest) bool {
		saved = prs
		return true
	})).Return(nil)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{recentPR, stalePR}, lastCheckTime)
	require.NoError(t, err)

	require.Len(t, saved, 2, "both recent and stale PRs must be saved")
	byURL := make(map[string]*pullrequest.PullRequest, len(saved))
	for _, pr := range saved {
		byURL[pr.URL()] = pr
	}

	require.Contains(t, byURL, recentPR.URL(), "recent PR must be in saved set")
	require.Contains(t, byURL, stalePR.URL(), "stale PR must be preserved in saved set")

	assert.Equal(t, "stale-sha", byURL[stalePR.URL()].HeadCommitSHA(), "stale PR enrichment state must be preserved")
	assert.Equal(t, pullrequest.PipelineStatusSuccess, byURL[stalePR.URL()].PipelineStatus(), "stale PR pipeline status must be preserved")
}
