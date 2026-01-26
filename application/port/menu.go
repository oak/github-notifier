package port

import (
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/domain/tracking"
)

// MenuPort is the port for updating the UI menu
type MenuPort interface {
	// UpdateMenu updates the menu with pull requests
	UpdateMenu(requestedReviewPRs, userCreatedPRs []*pullrequest.PullRequest, trackingService tracking.Service)
}
