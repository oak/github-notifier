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
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{}, time.Now())

	// Assert
	require.NoError(t, err)
	// No calls should be made
	mockPRRepo.AssertNotCalled(t, "EnrichWithActivities")
	mockTrackingRepo.AssertNotCalled(t, "LoadAll")
}

func TestTrackActivity_NoPRsDueForCheck(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	// Create stale PRs that were just checked
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-96*time.Hour)))
	prs := []*pullrequest.PullRequest{pr1, pr2}

	// Mark them as already checked at now
	scheduler.MarkCheckedAt(now, prs)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), prs, now.Add(-1*time.Hour))

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertNotCalled(t, "EnrichWithActivities")
	mockTrackingRepo.AssertNotCalled(t, "LoadAll")
}

func TestTrackActivity_NoNewActivity(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	// Create recent PRs (will be checked)
	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return(nil, nil)
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), prs, lastCheckTime)

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockSeenRepo.AssertNotCalled(t, "UnmarkAsSeen")
	mockEventPublisher.AssertNotCalled(t, "Publish")
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_NewActivity_EmitsEvents(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	// Create recent PRs
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-15*time.Minute)))
	prs := []*pullrequest.PullRequest{pr1, pr2}

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	// Mock EnrichWithActivities to add activities and return the resulting events.
	// Run() populates enrichedEvents from AddActivities; the return function provides
	// them to the use case so it can publish without needing DrainEvents.
	var enrichedEvents []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).
		Run(func(args mock.Arguments) {
			prsArg := args.Get(0).([]*pullrequest.PullRequest)
			for _, pr := range prsArg {
				activity := testutil.NewTestActivity(
					pullrequest.ActivityTypeComment,
					now.Add(-30*time.Minute), // Activity is after lastCheckTime
					testutil.WithActivityPR(pr.URL(), pr.Number()),
				)
				enrichedEvents = append(enrichedEvents, pr.AddActivities([]*pullrequest.Activity{activity})...)
			}
		}).
		Return(
			func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return enrichedEvents },
			nil,
		)

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// PRs with new activity should be marked as unseen
	mockSeenRepo.On("UnmarkAsSeen", pr1.Identifier()).Return(nil)
	mockSeenRepo.On("UnmarkAsSeen", pr2.Identifier()).Return(nil)

	// Events should be published
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Twice()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), prs, lastCheckTime)

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockSeenRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_EnrichError(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	prs := testutil.CreateTestPRs(2, 0)

	expectedErr := errors.New("github api error")

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return(nil, expectedErr)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), prs, lastCheckTime)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockSeenRepo.AssertNotCalled(t, "UnmarkAsSeen")
	mockEventPublisher.AssertNotCalled(t, "Publish")
	mockTrackingRepo.AssertNotCalled(t, "Save")
}

func TestTrackActivity_MarkUnseenError_ContinuesProcessing(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-15*time.Minute)))
	prs := []*pullrequest.PullRequest{pr1, pr2}

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	// Mock EnrichWithActivities to add activities and return the resulting events.
	var enrichedEvents2 []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).
		Run(func(args mock.Arguments) {
			prsArg := args.Get(0).([]*pullrequest.PullRequest)
			for _, pr := range prsArg {
				activity := testutil.NewTestActivity(
					pullrequest.ActivityTypeComment,
					now.Add(-30*time.Minute),
					testutil.WithActivityPR(pr.URL(), pr.Number()),
				)
				enrichedEvents2 = append(enrichedEvents2, pr.AddActivities([]*pullrequest.Activity{activity})...)
			}
		}).
		Return(
			func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return enrichedEvents2 },
			nil,
		)

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	mockSeenRepo.On("UnmarkAsSeen", pr1.Identifier()).Return(errors.New("tracking error"))
	mockSeenRepo.On("UnmarkAsSeen", pr2.Identifier()).Return(nil)

	// Events should still be published even if marking fails
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Twice()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), prs, lastCheckTime)

	// Assert
	require.NoError(t, err) // Use case doesn't return error on marking failure
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_PublishEventError_ContinuesProcessing(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-15*time.Minute)))
	prs := []*pullrequest.PullRequest{pr1, pr2}

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	var enrichedEvents3 []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Run(func(args mock.Arguments) {
		prsArg := args.Get(0).([]*pullrequest.PullRequest)
		for _, pr := range prsArg {
			activity := testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr.URL(), pr.Number()),
			)
			enrichedEvents3 = append(enrichedEvents3, pr.AddActivities([]*pullrequest.Activity{activity})...)
		}
	}).
		Return(
			func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return enrichedEvents3 },
			nil,
		)

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	mockSeenRepo.On("UnmarkAsSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(nil).Twice()

	// First event fails, second succeeds
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(errors.New("event bus error")).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), prs, lastCheckTime)

	// Assert
	require.NoError(t, err) // Use case doesn't return error on event failure
	mockEventPublisher.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_TwoTierScheduling(t *testing.T) {
	// Arrange
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	// Recent PR (< 48h old) - should always be checked
	recentPR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-24*time.Hour)))

	// Stale PR (> 48h old) - should be checked on first call
	stalePR := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-72*time.Hour)))

	prs := []*pullrequest.PullRequest{recentPR, stalePR}

	// First execution: both PRs are checked
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return(nil, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act - First execution
	err := uc.Execute(context.Background(), prs, lastCheckTime)

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)

	// Act - Second execution immediately after (stale PR shouldn't be checked again)
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", mock.MatchedBy(func(prsToCheck []*pullrequest.PullRequest) bool {
		// Only recent PR should be checked
		return len(prsToCheck) == 1 && prsToCheck[0].Number() == 1
	}), lastCheckTime).Return(nil, nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	err = uc.Execute(context.Background(), prs, lastCheckTime)

	// Assert
	require.NoError(t, err)
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_LoadsAndSeedsStateFromRepository(t *testing.T) {
	// Arrange — verify that enrichment state loaded from the tracking repo is
	// correctly applied to PR objects before EnrichWithActivities is called.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))

	prevSnap := pr1.ToSnapshot()
	prevSnap.HeadCommitSHA = "abc123"
	prevSnap.PipelineStatus = pullrequest.PipelineStatusSuccess

	// Return previous snapshot so Execute seeds the PR with known state
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{prevSnap}, nil).Once()

	var seededSHA string
	var seededStatus pullrequest.PipelineStatus
	mockPRRepo.On("EnrichWithActivities", mock.MatchedBy(func(prsToCheck []*pullrequest.PullRequest) bool {
		// Capture the state that was seeded onto the PR before enrichment
		seededSHA = prsToCheck[0].HeadCommitSHA()
		seededStatus = prsToCheck[0].PipelineStatus()
		return true
	}), lastCheckTime).Return(nil, nil).Once()

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr1}, lastCheckTime)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "abc123", seededSHA, "HeadCommitSHA should be seeded from stored snapshot")
	assert.Equal(t, pullrequest.PipelineStatusSuccess, seededStatus, "PipelineStatus should be seeded from stored snapshot")
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_PersistsLastActivityCheckAfterExecution(t *testing.T) {
	// Regression test for: LastActivityCheck always zero in persisted snapshots.
	// Execute must stamp a non-zero timestamp onto checked PRs so that after a
	// restart the scheduler can seed its state and avoid redundant re-checks.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", mock.Anything, lastCheckTime).Return(nil, nil).Once()

	var savedSnapshots []pullrequest.PRStateSnapshot
	mockTrackingRepo.On("Save", mock.MatchedBy(func(snaps []pullrequest.PRStateSnapshot) bool {
		savedSnapshots = snaps
		return true
	})).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	beforeExec := time.Now()
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr1}, lastCheckTime)
	afterExec := time.Now()

	require.NoError(t, err)
	require.Len(t, savedSnapshots, 1)
	lac := savedSnapshots[0].LastActivityCheck
	assert.False(t, lac.IsZero(), "LastActivityCheck must be non-zero after Execute")
	assert.True(t, !lac.Before(beforeExec) && !lac.After(afterExec),
		"LastActivityCheck should be within the execution window")
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_IgnoredAuthor_NotMarkedUnseen(t *testing.T) {
	// Verify that activity authored by an ignored user does NOT mark the PR as
	// unseen, even though the activity is by someone other than the authenticated user.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	pr := testutil.NewTestPullRequest(1,
		testutil.WithCreatedAt(now.Add(-10*time.Minute)),
		testutil.WithRepository("owner/repo"),
	)

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	var enrichedEvents []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", mock.Anything, lastCheckTime).
		Run(func(args mock.Arguments) {
			prsArg := args.Get(0).([]*pullrequest.PullRequest)
			// Activity by the ignored bot — not the authenticated user
			activity := testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(prsArg[0].URL(), prsArg[0].Number()),
				testutil.WithActivityAuthor("dependabot"),
			)
			enrichedEvents = append(enrichedEvents, prsArg[0].AddActivities([]*pullrequest.Activity{activity})...)
		}).
		Return(func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return enrichedEvents }, nil)

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "alice")

	// Configure ignore rules that suppress all activity from "dependabot"
	ignoreCfg := &pullrequest.IgnoreConfig{}
	ignoreCfg.Ignore.Global.AuthoredBy = []pullrequest.IgnoreActorRule{
		{Login: "dependabot"},
	}
	uc.UpdateIgnoreConfig(ignoreCfg)

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr}, lastCheckTime)

	require.NoError(t, err)
	// PR must NOT be marked unseen because the only activity is from an ignored author
	mockSeenRepo.AssertNotCalled(t, "UnmarkAsSeen")
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_IgnoreConfig_NonIgnoredAuthorStillMarkUnseen(t *testing.T) {
	// Ensure that when an ignore config is set, activity from non-ignored authors
	// still marks the PR as unseen.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	pr := testutil.NewTestPullRequest(1,
		testutil.WithCreatedAt(now.Add(-10*time.Minute)),
		testutil.WithRepository("owner/repo"),
	)

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	var enrichedEvents []pullrequest.Event
	mockPRRepo.On("EnrichWithActivities", mock.Anything, lastCheckTime).
		Run(func(args mock.Arguments) {
			prsArg := args.Get(0).([]*pullrequest.PullRequest)
			// Activity by a regular user — not ignored
			activity := testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(prsArg[0].URL(), prsArg[0].Number()),
				testutil.WithActivityAuthor("bob"),
			)
			enrichedEvents = append(enrichedEvents, prsArg[0].AddActivities([]*pullrequest.Activity{activity})...)
		}).
		Return(func([]*pullrequest.PullRequest, time.Time) []pullrequest.Event { return enrichedEvents }, nil)

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil)
	mockSeenRepo.On("UnmarkAsSeen", pr.Identifier()).Return(nil)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "alice")

	// Ignore config only suppresses "dependabot" — "bob" is not ignored
	ignoreCfg := &pullrequest.IgnoreConfig{}
	ignoreCfg.Ignore.Global.AuthoredBy = []pullrequest.IgnoreActorRule{
		{Login: "dependabot"},
	}
	uc.UpdateIgnoreConfig(ignoreCfg)

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr}, lastCheckTime)

	require.NoError(t, err)
	mockSeenRepo.AssertExpectations(t)
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}

func TestTrackActivity_RestartScenario_StaleCheckedOnceNotForever(t *testing.T) {
	// Regression test for the restart scenario.
	// Without the fix, LastActivityCheck is persisted as zero → SeedLastChecked
	// ignores it → scheduler always resets → stale PR re-checked every restart.
	//
	// With the fix: cycle 1 post-restart checks stale PR (scheduling before seeding
	// is unavoidable), but persists a non-zero LastActivityCheck. A fresh scheduler
	// seeded with that value will correctly skip the stale PR next cycle.
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockTrackingRepo := mocks.NewPRTrackingRepository(t)
	mockSeenRepo := mocks.NewSeenRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)

	// Fresh scheduler — simulates process restart.
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	stalePR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))

	// Cycle 1 post-restart: stale PR is checked (scheduler empty, can't skip).
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", mock.Anything, lastCheckTime).Return(nil, nil).Once()

	var savedAfterCycle1 []pullrequest.PRStateSnapshot
	mockTrackingRepo.On("Save", mock.MatchedBy(func(snaps []pullrequest.PRStateSnapshot) bool {
		savedAfterCycle1 = snaps
		return true
	})).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, mockSeenRepo, mockEventPublisher, "")

	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{stalePR}, lastCheckTime)
	require.NoError(t, err)

	// The persisted snapshot must have a non-zero LastActivityCheck.
	require.Len(t, savedAfterCycle1, 1)
	assert.False(t, savedAfterCycle1[0].LastActivityCheck.IsZero(),
		"LastActivityCheck must be persisted non-zero so a subsequent restart can seed the scheduler")

	// Cycle 2 (same process, same scheduler): stale PR must be skipped now
	// because MarkCheckedAt was called during cycle 1.
	// No LoadAll expected — use case returns early when 0 PRs are due.
	err = uc.Execute(context.Background(), []*pullrequest.PullRequest{stalePR}, lastCheckTime)
	require.NoError(t, err)

	// EnrichWithActivities must NOT be called in cycle 2 — stale PR recently checked.
	mockPRRepo.AssertNumberOfCalls(t, "EnrichWithActivities", 1)
	mockTrackingRepo.AssertExpectations(t)
}
