package macos

import (
	"fmt"

	"github.com/deckarep/gosx-notifier"
	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Adapter implements the NotificationPort interface for macOS
type Adapter struct {
	themeProvider ThemeProvider
}

// ThemeProvider provides the current system theme
type ThemeProvider interface {
	GetSystemTheme() string
}

// NewAdapter creates a new macOS notification adapter
func NewAdapter(themeProvider ThemeProvider) *Adapter {
	return &Adapter{
		themeProvider: themeProvider,
	}
}

// NotifyNewPullRequests sends a notification about new pull requests with click action
func (a *Adapter) NotifyNewPullRequests(title string, prs []*pullrequest.PullRequest) error {
	if len(prs) == 0 {
		return nil
	}

	message := fmt.Sprintf("%s: %d", title, len(prs))
	prList := ""
	for _, pr := range prs {
		prList += fmt.Sprintf("\n%s #%d", pr.RepositoryName(), pr.Number())
	}

	// For single PR, open it on click. For multiple PRs, open the first one
	var urlToOpen string
	if len(prs) == 1 {
		urlToOpen = prs[0].URL()
	} else if len(prs) > 1 {
		// For multiple PRs, could open first one or a GitHub search
		urlToOpen = prs[0].URL()
		prList += "\n\nClick to open first PR"
	}

	note := gosxnotifier.NewNotification(message + prList)
	note.Title = "GitHub Notifier"
	note.Sender = "com.apple.Safari" // Makes it look like Safari notification
	note.Sound = gosxnotifier.Default

	// Set up click action to open PR URL when notification is clicked
	if urlToOpen != "" {
		note.Link = urlToOpen
	}

	if err := note.Push(); err != nil {
		log.Error().Err(err).Msg("Error sending macOS notification")
		return err
	}

	return nil
}

// SupportsClickActions returns true for macOS adapter
func (a *Adapter) SupportsClickActions() bool {
	return true
}
