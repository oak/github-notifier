package port

import (
	"github.com/oak/github-notifier/domain/pullrequest"
)

// UIPort represents the boundary between the application and any UI implementation
// This port is UI-agnostic and can be implemented by systray menus, web UIs, terminal UIs, etc.
type UIPort interface {
	// UpdateDisplay updates the user interface with the current PR state
	// Different implementations can render this data however they want:
	// - Systray menu adapter: Shows PRs in dropdown menus
	// - Web UI adapter: Renders HTML with PR cards
	// - Terminal UI adapter: Displays in a TUI with lists
	UpdateDisplay(requestedReviewPRs, userCreatedPRs []*pullrequest.PullRequest, trackingRepo pullrequest.PRTrackingRepository)
}
