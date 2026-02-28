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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	// Create stale PRs that were just checked
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-96*time.Hour)))
	prs := []*pullrequest.PullRequest{pr1, pr2}

	// Mark them as already checked
	scheduler.MarkChecked(prs)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	// Create recent PRs (will be checked)
	prs := testutil.CreateTestPRs(2, 0)

	// Mock expectations
	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return(nil)
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	// Create recent PRs
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-15*time.Minute)))
	prs := []*pullrequest.PullRequest{pr1, pr2}

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	// Mock EnrichWithActivities to add activities
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Run(func(args mock.Arguments) {
		// Simulate adding activities to PRs
		prsArg := args.Get(0).([]*pullrequest.PullRequest)
		for _, pr := range prsArg {
			activity := testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute), // Activity is after lastCheckTime
				testutil.WithActivityPR(pr.URL(), pr.Number()),
			)
			pr.AddActivities([]*pullrequest.Activity{activity})
		}
	}).Return(nil)

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// PRs with new activity should be marked as unseen
	mockSeenRepo.On("UnmarkAsSeen", pr1.Identifier()).Return(nil)
	mockSeenRepo.On("UnmarkAsSeen", pr2.Identifier()).Return(nil)

	// Events should be published
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Twice()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)
	prs := testutil.CreateTestPRs(2, 0)

	expectedErr := errors.New("github api error")

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return(expectedErr)

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-15*time.Minute)))
	prs := []*pullrequest.PullRequest{pr1, pr2}

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	// Mock EnrichWithActivities to add activities
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Run(func(args mock.Arguments) {
		prsArg := args.Get(0).([]*pullrequest.PullRequest)
		for _, pr := range prsArg {
			activity := testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr.URL(), pr.Number()),
			)
			pr.AddActivities([]*pullrequest.Activity{activity})
		}
	}).Return(nil)

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	// First UnmarkAsSeen fails, second succeeds
	mockSeenRepo.On("UnmarkAsSeen", pr1.Identifier()).Return(errors.New("tracking error"))
	mockSeenRepo.On("UnmarkAsSeen", pr2.Identifier()).Return(nil)

	// Events should still be published even if marking fails
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Twice()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockEventPublisher := mocks.NewEventPublisher(t)
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)

	now := time.Now()
	lastCheckTime := now.Add(-1 * time.Hour)

	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-15*time.Minute)))
	prs := []*pullrequest.PullRequest{pr1, pr2}

	mockTrackingRepo.On("LoadAll").Return([]pullrequest.PRStateSnapshot{}, nil).Once()

	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Run(func(args mock.Arguments) {
		prsArg := args.Get(0).([]*pullrequest.PullRequest)
		for _, pr := range prsArg {
			activity := testutil.NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-30*time.Minute),
				testutil.WithActivityPR(pr.URL(), pr.Number()),
			)
			pr.AddActivities([]*pullrequest.Activity{activity})
		}
	}).Return(nil)

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	mockSeenRepo.On("UnmarkAsSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(nil).Twice()

	// First event fails, second succeeds
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(errors.New("event bus error")).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.ActivityDetected")).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
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
	mockPRRepo.On("EnrichWithActivities", prs, lastCheckTime).Return(nil).Once()
	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

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
	}), lastCheckTime).Return(nil).Once()
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
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
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
	}), lastCheckTime).Return(nil).Once()

	mockTrackingRepo.On("Save", mock.Anything).Return(nil).Once()

	uc := usecase.NewTrackPullRequestActivityUseCase(mockPRRepo, mockTrackingRepo, scheduler, trackingService, mockEventPublisher, "")

	// Act
	err := uc.Execute(context.Background(), []*pullrequest.PullRequest{pr1}, lastCheckTime)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "abc123", seededSHA, "HeadCommitSHA should be seeded from stored snapshot")
	assert.Equal(t, pullrequest.PipelineStatusSuccess, seededStatus, "PipelineStatus should be seeded from stored snapshot")
	mockPRRepo.AssertExpectations(t)
	mockTrackingRepo.AssertExpectations(t)
}
