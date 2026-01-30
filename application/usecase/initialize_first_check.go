package usecase

import (
	"log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/domain/tracking"
)

// InitializeFirstCheckUseCase handles the first-run initialization
// On first run, all existing PRs are marked as seen to avoid notifications
type InitializeFirstCheckUseCase struct {
	prRepo          pullrequest.PullRequestRepository
	trackingService tracking.Service
	prFilter        *pullrequest.PRFilter
	uiPort          port.UIPort
}

// NewInitializeFirstCheckUseCase creates a new use case
func NewInitializeFirstCheckUseCase(
	prRepo pullrequest.PullRequestRepository,
	trackingService tracking.Service,
	prFilter *pullrequest.PRFilter,
	uiPort port.UIPort,
) *InitializeFirstCheckUseCase {
	return &InitializeFirstCheckUseCase{
		prRepo:          prRepo,
		trackingService: trackingService,
		prFilter:        prFilter,
		uiPort:          uiPort,
	}
}

// Execute runs the first-run initialization
// Returns true if this was the first run (tracking service was empty)
func (uc *InitializeFirstCheckUseCase) Execute() (bool, error) {
	// Check if this is truly the first run ever
	isFirstRunEver := uc.trackingService.IsEmpty()

	if !isFirstRunEver {
		return false, nil
	}

	log.Println("First run detected - marking all existing PRs as already seen")

	// Fetch all PRs
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Printf("Error fetching requested review PRs: %v", err)
		return false, err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Printf("Error fetching user created PRs: %v", err)
		return false, err
	}

	// Filter draft PRs if configured
	requestedReviewPRs = uc.prFilter.FilterDrafts(requestedReviewPRs)
	userCreatedPRs = uc.prFilter.FilterDrafts(userCreatedPRs)

	// Mark all existing PRs as seen (no notifications, no asterisks on first run)
	uc.trackingService.MarkPullRequestsAsSeen(requestedReviewPRs)
	uc.trackingService.MarkPullRequestsAsSeen(userCreatedPRs)

	// Update the UI with tracking service
	uc.uiPort.UpdateDisplay(requestedReviewPRs, userCreatedPRs, uc.trackingService)

	log.Printf("First run complete: marked %d PRs as seen",
		len(requestedReviewPRs)+len(userCreatedPRs))

	return true, nil
}
