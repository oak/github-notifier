package usecase

import (
	"context"
	"sort"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// UpdatePullRequestDisplayUseCase handles updating the UI with current PR state
type UpdatePullRequestDisplayUseCase struct {
	uiPort         port.UIPort
	prTrackingRepo pullrequest.PRTrackingRepository
}

// NewUpdatePullRequestDisplayUseCase creates a new use case
func NewUpdatePullRequestDisplayUseCase(
	uiPort port.UIPort,
	prTrackingRepo pullrequest.PRTrackingRepository,
) *UpdatePullRequestDisplayUseCase {
	return &UpdatePullRequestDisplayUseCase{
		uiPort:         uiPort,
		prTrackingRepo: prTrackingRepo,
	}
}

// Execute updates the UI display with the given PRs
// PRs are sorted by creation date (oldest first) before display
func (uc *UpdatePullRequestDisplayUseCase) Execute(ctx context.Context, requestedReviewPRs []*pullrequest.PullRequest,
	userCreatedPRs []*pullrequest.PullRequest,
) error {
	// Sort PRs by creation date (oldest first) — original slices are not modified
	uc.uiPort.UpdateDisplay(sortedByCreatedAt(requestedReviewPRs), sortedByCreatedAt(userCreatedPRs), uc.prTrackingRepo)

	return nil
}

// sortedByCreatedAt returns a new slice sorted by creation date (oldest first)
func sortedByCreatedAt(prs []*pullrequest.PullRequest) []*pullrequest.PullRequest {
	out := make([]*pullrequest.PullRequest, len(prs))
	copy(out, prs)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt().Before(out[j].CreatedAt())
	})
	return out
}
