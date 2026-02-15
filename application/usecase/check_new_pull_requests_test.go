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

func TestCheckNewPRs_NoNewPRs(t *testing.T) {
	// Arrange
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()

	requestedPRs := testutil.CreateTestPRs(2, 0)
	userPRs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(10, testutil.WithURL("https://github.com/owner/repo/pull/10")),
	}

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(userPRs, nil)

	// All PRs are new (not seen before)
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(false).Times(3)
	mockSeenRepo.On("MarkAsSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(nil).Times(3)

	// Events should be published for each PR (2 + 1 = 3 events)
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Times(3)

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)

	// Act
	result, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.RequestedReviewPRs, 2)
	assert.Len(t, result.UserCreatedPRs, 1)
	mockEventPublisher.AssertExpectations(t)
	mockSeenRepo.AssertExpectations(t)
}

func TestCheckNewPRs_TrulyNewPRs_EmitsEvents(t *testing.T) {
	// Arrange
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()

	requestedPRs := testutil.CreateTestPRs(2, 0)
	userPRs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(10, testutil.WithURL("https://github.com/owner/repo/pull/10")),
	}

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(userPRs, nil)

	// All PRs are new
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(false).Times(3)

	// All new PRs should be marked as seen
	mockSeenRepo.On("MarkAsSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(nil).Times(3)

	// Events should be published for each PR (2 + 1 = 3 events)
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Times(3)

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)

	// Act
	result, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	mockEventPublisher.AssertExpectations(t)
	mockSeenRepo.AssertExpectations(t)
}

func TestCheckNewPRs_PRsWithActivity(t *testing.T) {
	// Arrange
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()

	// Create use case first to establish lastCheckTime
	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)

	// Sleep briefly to ensure activities are after lastCheckTime
	time.Sleep(10 * time.Millisecond)

	// Create PRs with activities AFTER the use case was created
	// so they will be detected as "with activity"
	now := time.Now()
	pr1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-1*time.Hour)))

	// Add activities that are recent (now)
	activity1 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now, testutil.WithActivityPR(pr1.URL(), pr1.Number()))
	activity2 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now, testutil.WithActivityPR(pr2.URL(), pr2.Number()))
	pr1.AddActivities([]*pullrequest.Activity{activity1})
	pr2.AddActivities([]*pullrequest.Activity{activity2})

	prsWithActivity := []*pullrequest.PullRequest{pr1, pr2}

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(prsWithActivity, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)

	// PRs are found as new
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(true)

	// PRs with activity should NOT be marked as seen (handled by activity tracking)
	// No events should be published for PRs with activity

	// Act
	result, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	mockEventPublisher.AssertNotCalled(t, "Publish") // No events for PRs with activity
	mockSeenRepo.AssertNotCalled(t, "MarkAsSeen")    // Not marked as seen
}

func TestCheckNewPRs_MixedNewAndActivity(t *testing.T) {
	// Arrange
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()

	// Create use case first
	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)

	// Sleep briefly
	time.Sleep(10 * time.Millisecond)

	now := time.Now()

	// 2 truly new PRs (no activities) - created recently
	newPR1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"), testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	newPR2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"), testutil.WithCreatedAt(now.Add(-10*time.Minute)))
	newPRs := []*pullrequest.PullRequest{newPR1, newPR2}

	// 2 PRs with recent activities - created 1 hour ago with activities now
	activePR1 := testutil.NewTestPullRequest(3, testutil.WithURL("https://github.com/owner/repo/pull/3"), testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	activePR2 := testutil.NewTestPullRequest(4, testutil.WithURL("https://github.com/owner/repo/pull/4"), testutil.WithCreatedAt(now.Add(-1*time.Hour)))
	activity1 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now, testutil.WithActivityPR(activePR1.URL(), activePR1.Number()))
	activity2 := testutil.NewTestActivity(pullrequest.ActivityTypeComment, now, testutil.WithActivityPR(activePR2.URL(), activePR2.Number()))
	activePR1.AddActivities([]*pullrequest.Activity{activity1})
	activePR2.AddActivities([]*pullrequest.Activity{activity2})
	activePRs := []*pullrequest.PullRequest{activePR1, activePR2}

	allPRs := append(newPRs, activePRs...)

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(allPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)

	// All 4 PRs are not seen yet
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(false)

	// All new PRs should be marked as seen (both truly new and those with activity)
	mockSeenRepo.On("MarkAsSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(nil).Times(4)

	// Events only for truly new PRs (2 events)
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Twice()

	// Act
	result, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.RequestedReviewPRs, 4)
	mockEventPublisher.AssertExpectations(t)
	mockSeenRepo.AssertExpectations(t)
}

func TestCheckNewPRs_FetchRequestedReviewsError(t *testing.T) {
	// Arrange
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()

	expectedErr := errors.New("github api error")

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(nil, expectedErr)

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)

	// Act
	result, err := uc.Execute(context.Background())

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
	mockPRRepo.AssertNotCalled(t, "FetchUserCreated")
}

func TestCheckNewPRs_FetchUserCreatedError(t *testing.T) {
	// Arrange
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()

	requestedPRs := testutil.CreateTestPRs(2, 0)
	expectedErr := errors.New("github api error")

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return(nil, expectedErr)

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)

	// Act
	result, err := uc.Execute(context.Background())

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
}

func TestCheckNewPRs_FiltersDrafts(t *testing.T) {
	// Arrange
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false) // exclude drafts
	prClassifier := pullrequest.NewPRClassifier()

	// 2 regular, 2 drafts
	requestedPRs := testutil.CreateTestPRs(2, 2)

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(requestedPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)

	// Only 2 non-draft PRs should be checked (drafts are filtered out)
	// Both non-draft PRs are new
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(false).Twice()
	mockSeenRepo.On("MarkAsSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(nil).Twice()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Twice()

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)

	// Act
	result, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Len(t, result.RequestedReviewPRs, 2, "Should filter out drafts")
	testutil.AssertNoDrafts(t, result.RequestedReviewPRs)
}

func TestCheckNewPRs_PublishEventError_ContinuesProcessing(t *testing.T) {
	// Arrange
	mockSeenRepo := mocks.NewSeenRepository(t)
	trackingService := pullrequest.NewTrackingService(mockSeenRepo)
	mockPRRepo := mocks.NewPullRequestRepository(t)
	mockEventPublisher := mocks.NewEventPublisher(t)
	prFilter := pullrequest.NewPRFilter(false)
	prClassifier := pullrequest.NewPRClassifier()

	newPRs := []*pullrequest.PullRequest{
		testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1")),
		testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2")),
	}

	// Mock expectations
	mockPRRepo.On("FetchRequestedReviews").Return(newPRs, nil)
	mockPRRepo.On("FetchUserCreated").Return([]*pullrequest.PullRequest{}, nil)

	// Both PRs are new
	mockSeenRepo.On("HasBeenSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(false).Twice()

	// First event fails, second succeeds
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(errors.New("event bus error")).Once()
	mockEventPublisher.On("Publish", mock.AnythingOfType("*pullrequest.NewPullRequestDetected")).Return(nil).Once()

	// PRs should still be marked as seen even if event fails
	mockSeenRepo.On("MarkAsSeen", mock.AnythingOfType("pullrequest.PRIdentifier")).Return(nil).Twice()

	uc := usecase.NewCheckNewPullRequestsUseCase(mockPRRepo, trackingService, prFilter, prClassifier, mockEventPublisher)

	// Act
	result, err := uc.Execute(context.Background())

	// Assert
	require.NoError(t, err) // Use case doesn't return error on event publish failure
	assert.NotNil(t, result)
	mockEventPublisher.AssertExpectations(t)
	mockSeenRepo.AssertExpectations(t)
}
