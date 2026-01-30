package usecase

import (
	"sort"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/domain/tracking"
)

// UpdatePullRequestDisplayUseCase handles updating the UI with current PR state
type UpdatePullRequestDisplayUseCase struct {
	uiPort          port.UIPort
	trackingService tracking.Service
}

// NewUpdatePullRequestDisplayUseCase creates a new use case
func NewUpdatePullRequestDisplayUseCase(
	uiPort port.UIPort,
	trackingService tracking.Service,
) *UpdatePullRequestDisplayUseCase {
	return &UpdatePullRequestDisplayUseCase{
		uiPort:          uiPort,
		trackingService: trackingService,
	}
}

// Execute updates the UI display with the given PRs
// PRs are sorted by creation date (oldest first) before display
func (uc *UpdatePullRequestDisplayUseCase) Execute(
	requestedReviewPRs []*pullrequest.PullRequest,
	userCreatedPRs []*pullrequest.PullRequest,
) error {
	// Sort PRs by creation date (oldest first)
	uc.sortPRsByCreatedAt(requestedReviewPRs)
	uc.sortPRsByCreatedAt(userCreatedPRs)

	// Update UI with sorted PRs and tracking state
	uc.uiPort.UpdateDisplay(requestedReviewPRs, userCreatedPRs, uc.trackingService)

	return nil
}

// sortPRsByCreatedAt sorts pull requests by creation date (oldest first)
func (uc *UpdatePullRequestDisplayUseCase) sortPRsByCreatedAt(prs []*pullrequest.PullRequest) {
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].CreatedAt().Before(prs[j].CreatedAt())
	})
}
