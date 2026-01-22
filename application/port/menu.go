package port

import "github.com/oak3/github-notifier/domain/pullrequest"

// MenuPort is the port for updating the UI menu
type MenuPort interface {
	// UpdateMenu updates the menu with pull requests
	UpdateMenu(requestedReviewPRs, userCreatedPRs []*pullrequest.PullRequest)
}
