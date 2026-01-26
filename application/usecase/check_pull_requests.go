package usecase

import (
	"log"
	"sort"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/domain/tracking"
)

// CheckPullRequestsUseCase handles checking for pull requests and updating the UI
type CheckPullRequestsUseCase struct {
	prRepo           pullrequest.Repository
	trackingService  tracking.Service
	notificationPort port.NotificationPort
	menuPort         port.MenuPort
}

// NewCheckPullRequestsUseCase creates a new use case
func NewCheckPullRequestsUseCase(
	prRepo pullrequest.Repository,
	trackingService tracking.Service,
	notificationPort port.NotificationPort,
	menuPort port.MenuPort,
) *CheckPullRequestsUseCase {
	return &CheckPullRequestsUseCase{
		prRepo:           prRepo,
		trackingService:  trackingService,
		notificationPort: notificationPort,
		menuPort:         menuPort,
	}
}

// ExecuteInitial runs the use case on app startup
// If this is the very first run (seen repository is empty), mark all PRs as seen
// to avoid notifications and asterisks for PRs that existed before the app started.
// If the repository has data (e.g., Postgres with persistent state), behave like Execute()
func (uc *CheckPullRequestsUseCase) ExecuteInitial() error {
	// Check if this is truly the first run ever (seen repository is empty)
	isFirstRunEver := uc.trackingService.IsEmpty()

	// Fetch pull requests
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Printf("Error fetching requested review PRs: %v", err)
		return err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Printf("Error fetching user created PRs: %v", err)
		return err
	}

	// Sort PRs by creation date (oldest first)
	uc.sortPRsByCreatedAt(requestedReviewPRs)
	uc.sortPRsByCreatedAt(userCreatedPRs)

	if isFirstRunEver {
		// First run ever: mark all existing PRs as seen (no notifications, no asterisks)
		log.Println("First run detected - marking all existing PRs as already seen")
		uc.trackingService.MarkPullRequestsAsSeen(requestedReviewPRs)
		uc.trackingService.MarkPullRequestsAsSeen(userCreatedPRs)

		// Update the menu with tracking service
		uc.menuPort.UpdateMenu(requestedReviewPRs, userCreatedPRs, uc.trackingService)
	} else {
		// Not first run: repository has data, behave normally
		log.Println("Existing state detected - checking for new PRs")

		// Update the menu first
		uc.menuPort.UpdateMenu(requestedReviewPRs, userCreatedPRs, uc.trackingService)

		// Track and notify new PRs (same logic as Execute)
		newRequestedReviewPRs := uc.trackingService.FindNewPullRequests(requestedReviewPRs)
		if len(newRequestedReviewPRs) > 0 {
			err := uc.notificationPort.NotifyNewPullRequests("New PRs needing review", newRequestedReviewPRs)
			if err != nil {
				log.Printf("Error sending notification for requested review PRs: %v", err)
			}
			uc.trackingService.MarkPullRequestsAsSeen(newRequestedReviewPRs)
		}

		newUserCreatedPRs := uc.trackingService.FindNewPullRequests(userCreatedPRs)
		if len(newUserCreatedPRs) > 0 {
			err := uc.notificationPort.NotifyNewPullRequests("New PRs by you", newUserCreatedPRs)
			if err != nil {
				log.Printf("Error sending notification for user created PRs: %v", err)
			}
			uc.trackingService.MarkPullRequestsAsSeen(newUserCreatedPRs)
		}
	}

	return nil
}

// Execute runs the use case
func (uc *CheckPullRequestsUseCase) Execute() error {
	// Fetch pull requests
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Printf("Error fetching requested review PRs: %v", err)
		return err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Printf("Error fetching user created PRs: %v", err)
		return err
	}

	// Sort PRs by creation date (oldest first)
	uc.sortPRsByCreatedAt(requestedReviewPRs)
	uc.sortPRsByCreatedAt(userCreatedPRs)

	// Update the menu with tracking service
	uc.menuPort.UpdateMenu(requestedReviewPRs, userCreatedPRs, uc.trackingService)

	// Find new PRs for notification (without marking as seen yet)
	newRequestedReviewPRs := uc.trackingService.FindNewPullRequests(requestedReviewPRs)
	if len(newRequestedReviewPRs) > 0 {
		err := uc.notificationPort.NotifyNewPullRequests("New PRs needing review", newRequestedReviewPRs)
		if err != nil {
			log.Printf("Error sending notification for requested review PRs: %v", err)
		}
		// Only mark as seen after successful notification to avoid duplicate notifications
		// But user still needs to click them in the menu to remove asterisks
		uc.trackingService.MarkPullRequestsAsSeen(newRequestedReviewPRs)
	}

	newUserCreatedPRs := uc.trackingService.FindNewPullRequests(userCreatedPRs)
	if len(newUserCreatedPRs) > 0 {
		err := uc.notificationPort.NotifyNewPullRequests("New PRs by you", newUserCreatedPRs)
		if err != nil {
			log.Printf("Error sending notification for user created PRs: %v", err)
		}
		// Only mark as seen after successful notification to avoid duplicate notifications
		// But user still needs to click them in the menu to remove asterisks
		uc.trackingService.MarkPullRequestsAsSeen(newUserCreatedPRs)
	}

	return nil
}

// sortPRsByCreatedAt sorts pull requests by creation date (oldest first)
func (uc *CheckPullRequestsUseCase) sortPRsByCreatedAt(prs []*pullrequest.PullRequest) {
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].CreatedAt().Before(prs[j].CreatedAt())
	})
}
