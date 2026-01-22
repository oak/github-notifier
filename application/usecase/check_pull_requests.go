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

	// Update the menu
	uc.menuPort.UpdateMenu(requestedReviewPRs, userCreatedPRs)

	// Track and notify new PRs
	newRequestedReviewPRs := uc.trackingService.FindNewPullRequests(requestedReviewPRs)
	if len(newRequestedReviewPRs) > 0 {
		err := uc.notificationPort.NotifyNewPullRequests("New PRs needing review", newRequestedReviewPRs)
		if err != nil {
			log.Printf("Error sending notification for requested review PRs: %v", err)
		}
	}

	newUserCreatedPRs := uc.trackingService.FindNewPullRequests(userCreatedPRs)
	if len(newUserCreatedPRs) > 0 {
		err := uc.notificationPort.NotifyNewPullRequests("New PRs by you", newUserCreatedPRs)
		if err != nil {
			log.Printf("Error sending notification for user created PRs: %v", err)
		}
	}

	return nil
}

// sortPRsByCreatedAt sorts pull requests by creation date (oldest first)
func (uc *CheckPullRequestsUseCase) sortPRsByCreatedAt(prs []*pullrequest.PullRequest) {
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].CreatedAt().Before(prs[j].CreatedAt())
	})
}
