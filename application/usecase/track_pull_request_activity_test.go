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
	mockPRRepo.AssertNotCalled(t, "EnrichWithActivities")
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
	mockPRRepo.AssertNotCalled(t, "EnrichWithActivities")
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
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return([]pullrequest.Event{}, nil).Once()
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

	var enrichedEvents []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).
		Run(func(args mock.Arguments) {
			prsArg := args.Get(0).([]*pullrequest.PullRequest)
			for _, pr := range prsArg {
				activity := testutil.NewTestActivity(
					pullrequest.ActivityTypeComment,
					now.Add(-30*time.Minute),
					testutil.WithActivityPR(pr.URL(), pr.Number()),
				)
				enrichedEvents = append(enrichedEvents, pr.AddActivities([]*pullrequest.Activity{activity})...)
			}
		}).
		Return(func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return enrichedEvents }, nil)

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
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return([]pullrequest.Event{}, expectedErr).Once()

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

	var enrichedEvents []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).
		Run(func(args mock.Arguments) {
			prsArg := args.Get(0).([]*pullrequest.PullRequest)
			for _, pr := range prsArg {
				activity := testutil.NewTestActivity(
					pullrequest.ActivityTypeComment,
					now.Add(-30*time.Minute),
					testutil.WithActivityPR(pr.URL(), pr.Number()),
				)
				enrichedEvents = append(enrichedEvents, pr.AddActivities([]*pullrequest.Activity{activity})...)
			}
		}).
		Return(func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return enrichedEvents }, nil)

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
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return([]pullrequest.Event{}, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockEventPublisher, "")

	err := uc.Execute(context.Background(), prs, lastCheckTime)
	require.NoError(t, err)

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", mock.MatchedBy(func(prsToCheck []*pullrequest.PullRequest) bool {
		return len(prsToCheck) == 1 && prsToCheck[0].Number() == 1
	}), lastCheckTime).Return([]pullrequest.Event{}, nil).Once()
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
	prevPR.SetInitialHeadCommitSHA("abc123")
	prevPR.SetInitialPipelineStatus(pullrequest.PipelineStatusSuccess)

	mockTrackingRepo.On("LoadAll").Return([]*pullrequest.PullRequest{prevPR}, nil).Once()

	var seededSHA string
	var seededStatus pullrequest.PipelineStatus
	mockPRRepo.On("EnrichWithActivities", mock.MatchedBy(func(prsToCheck []*pullrequest.PullRequest) bool {
		seededSHA = prsToCheck[0].HeadCommitSHA()
		seededStatus = prsToCheck[0].PipelineStatus()
		return true
	}), lastCheckTime).Return([]pullrequest.Event{}, nil).Once()

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
	mockPRRepo.On("EnrichWithActivities", mock.Anything, lastCheckTime).Return([]pullrequest.Event{}, nil).Once()

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

	var events []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", mock.Anything, lastCheckTime).
		Run(func(args mock.Arguments) {
			prsArg := args.Get(0).([]*pullrequest.PullRequest)
			activity := testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(prsArg[0].URL(), prsArg[0].Number()),
				testutil.WithActivityAuthor("dependabot"),
			)
			events = append(events, prsArg[0].AddActivities([]*pullrequest.Activity{activity})...)
		}).
		Return(func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return events }, nil)

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

	var events []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", mock.Anything, lastCheckTime).
		Run(func(args mock.Arguments) {
			prsArg := args.Get(0).([]*pullrequest.PullRequest)
			activity := testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(prsArg[0].URL(), prsArg[0].Number()),
				testutil.WithActivityAuthor("bob"),
			)
			events = append(events, prsArg[0].AddActivities([]*pullrequest.Activity{activity})...)
		}).
		Return(func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return events }, nil)

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
	mockPRRepo.On("EnrichWithActivities", mock.Anything, lastCheckTime).Return([]pullrequest.Event{}, nil).Once()

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
	mockPRRepo.AssertNumberOfCalls(t, "EnrichWithActivities", 1)
}
