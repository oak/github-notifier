package macos

import (
	"fmt"
	"strings"

	gosxnotifier "github.com/deckarep/gosx-notifier"
	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Adapter implements the NotificationPort interface for macOS
type Adapter struct {
	themeProvider ThemeProvider
	sender        string
}

// ThemeProvider provides the current system theme
type ThemeProvider interface {
	GetSystemTheme() string
}

// NewAdapter creates a new macOS notification adapter
func NewAdapter(themeProvider ThemeProvider, sender string) *Adapter {
	return &Adapter{
		themeProvider: themeProvider,
		sender:        sender,
	}
}

// NotifyPullRequests sends grouped notifications for pull requests
func (a *Adapter) NotifyPullRequests(notifications []*port.PRNotificationData) error {
	if len(notifications) == 0 {
		return nil
	}

	// Send one notification per PR
	for _, prNotif := range notifications {
		if err := a.sendPRNotification(prNotif); err != nil {
			log.Error().Err(err).Msg("Error sending PR notification")
			// Continue sending other notifications
		}
	}

	return nil
}

// sendPRNotification sends a single PR notification with all its activities
func (a *Adapter) sendPRNotification(prNotif *port.PRNotificationData) error {
	pr := prNotif.PullRequest

	// Build the title
	title := a.buildNotificationTitle(prNotif)

	// Build the message
	message := a.buildNotificationMessage(prNotif)

	note := gosxnotifier.NewNotification(message)
	note.Title = title
	if a.sender != "" {
		note.Sender = a.sender
	}
	note.Sound = gosxnotifier.Default

	// Set up click action to open PR URL
	note.Link = pr.URL()

	if err := note.Push(); err != nil {
		log.Error().Err(err).Msg("Error sending macOS notification")
		return err
	}

	return nil
}

// buildNotificationTitle creates the title for a PR notification
func (a *Adapter) buildNotificationTitle(prNotif *port.PRNotificationData) string {
	pr := prNotif.PullRequest

	if prNotif.IsNew {
		return fmt.Sprintf("New PR #%d", pr.Number())
	}

	return fmt.Sprintf("PR #%d Activity", pr.Number())
}

// buildNotificationMessage creates the message for a PR notification
func (a *Adapter) buildNotificationMessage(prNotif *port.PRNotificationData) string {
	pr := prNotif.PullRequest
	var parts []string

	// Add PR info
	parts = append(parts, pr.RepositoryName())
	parts = append(parts, pr.Title())

	// Add "NEW" indicator if this is a new PR
	if prNotif.IsNew {
		parts = append(parts, "🆕 Needs review")
	}

	// Add activities
	if len(prNotif.Activities) > 0 {
		activityLines := []string{}

		// Sort by priority
		activityOrder := []pullrequest.ActivityType{
			pullrequest.ActivityTypePush,
			pullrequest.ActivityTypeReview,
			pullrequest.ActivityTypeComment,
			pullrequest.ActivityTypeReaction,
			pullrequest.ActivityTypeCommit,
		}

		for _, actType := range activityOrder {
			for _, activity := range prNotif.Activities {
				if activity.Type == actType {
					label := a.getActivityLabel(activity.Type, activity.Count)
					activityLines = append(activityLines, label)
				}
			}
		}

		if len(activityLines) > 0 {
			parts = append(parts, strings.Join(activityLines, "\n"))
		}
	}

	// Add status changes
	for _, statusChange := range prNotif.StatusChanges {
		if statusChange.EventType == "merged" {
			parts = append(parts, "✅ Merged")
		} else if statusChange.EventType == "closed" {
			parts = append(parts, "❌ Closed")
		}
	}

	return strings.Join(parts, "\n")
}

// getActivityLabel returns a formatted label for an activity
func (a *Adapter) getActivityLabel(actType pullrequest.ActivityType, count int) string {
	switch actType {
	case pullrequest.ActivityTypePush:
		if count == 1 {
			return "📤 1 new commit"
		}
		return fmt.Sprintf("📤 %d new commits", count)
	case pullrequest.ActivityTypeReview:
		if count == 1 {
			return "👁 1 new review"
		}
		return fmt.Sprintf("👁 %d new reviews", count)
	case pullrequest.ActivityTypeComment:
		if count == 1 {
			return "💬 1 new comment"
		}
		return fmt.Sprintf("💬 %d new comments", count)
	case pullrequest.ActivityTypeReaction:
		if count == 1 {
			return "👍 1 new reaction"
		}
		return fmt.Sprintf("👍 %d new reactions", count)
	case pullrequest.ActivityTypeCommit:
		if count == 1 {
			return "📝 1 new commit"
		}
		return fmt.Sprintf("📝 %d new commits", count)
	default:
		if count == 1 {
			return "• 1 new activity"
		}
		return fmt.Sprintf("• %d new activities", count)
	}
}

// NotifyNewPullRequests sends a notification about new pull requests with click action
// DEPRECATED: Use NotifyPullRequests instead
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
	if a.sender != "" {
		note.Sender = a.sender
	}
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
