package port

import (
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// PullRequestSeenReader exposes only the read-only seen-state query needed by UI adapters.
type PullRequestSeenReader interface {
	HasBeenSeen(id pullrequest.PRIdentifier) bool
}

// UIPort represents the boundary between the application and any UI implementation
// This port is UI-agnostic and can be implemented by systray menus, web UIs, terminal UIs, etc.
type UIPort interface {
	// UpdateDisplay updates the user interface with the current PR state
	// Different implementations can render this data however they want:
	// - Systray menu adapter: Shows PRs in dropdown menus
	// - Web UI adapter: Renders HTML with PR cards
	// - Terminal UI adapter: Displays in a TUI with lists
	UpdateDisplay(requestedReviewPRs, userCreatedPRs []*pullrequest.PullRequest, seenReader PullRequestSeenReader)
}
