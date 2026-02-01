package desktop

import (
	"fmt"

	"github.com/gen2brain/beeep"
	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/assets"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Adapter implements the NotificationPort interface
type Adapter struct {
	themeProvider ThemeProvider
}

// ThemeProvider provides the current system theme
type ThemeProvider interface {
	GetSystemTheme() string
}

// NewAdapter creates a new notification adapter
func NewAdapter(themeProvider ThemeProvider) *Adapter {
	return &Adapter{
		themeProvider: themeProvider,
	}
}

// NotifyNewPullRequests sends a notification about new pull requests
func (a *Adapter) NotifyNewPullRequests(title string, prs []*pullrequest.PullRequest) error {
	if len(prs) == 0 {
		return nil
	}

	message := fmt.Sprintf("%s: %d", title, len(prs))
	for _, pr := range prs {
		message += fmt.Sprintf("\n%s #%d", pr.RepositoryName(), pr.Number())
	}

	iconData := a.selectIcon()

	err := beeep.Notify("GitHub Notifier", message, iconData)
	if err != nil {
		log.Error().Msgf("Error sending notification: %v", err)
		return err
	}

	return nil
}

// selectIcon selects the appropriate icon based on system theme
func (a *Adapter) selectIcon() []byte {
	theme := a.themeProvider.GetSystemTheme()
	if theme == "dark" {
		return assets.GitPullRequestIcon
	}
	return assets.GitPullRequestLightIcon
}
