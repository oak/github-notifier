package desktop

import (
	"fmt"
	"strings"

	"github.com/gen2brain/beeep"
	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
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

// NotifyPullRequests sends grouped notifications for pull requests
func (a *Adapter) NotifyPullRequests(notifications []*port.PRNotificationData) error {
	if len(notifications) == 0 {
		return nil
	}

	iconData := a.selectIcon()

	// Send one notification per PR
	for _, prNotif := range notifications {
		title := a.buildNotificationTitle(prNotif)
		message := a.buildNotificationMessage(prNotif)

		err := beeep.Notify(title, message, iconData)
		if err != nil {
			log.Error().Msgf("Error sending notification: %v", err)
			// Continue sending other notifications
		}
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
		parts = append(parts, "Needs review")
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
			parts = append(parts, strings.Join(activityLines, ", "))
		}
	}

	// Add status changes
	for _, statusChange := range prNotif.StatusChanges {
		if statusChange.EventType == pullrequest.StatusChangeMerged {
			parts = append(parts, "Merged")
		} else if statusChange.EventType == pullrequest.StatusChangeClosed {
			parts = append(parts, "Closed")
		}
	}

	return strings.Join(parts, "\n")
}

// getActivityLabel returns a formatted label for an activity
func (a *Adapter) getActivityLabel(actType pullrequest.ActivityType, count int) string {
	switch actType {
	case pullrequest.ActivityTypePush:
		if count == 1 {
			return "1 new commit"
		}
		return fmt.Sprintf("%d new commits", count)
	case pullrequest.ActivityTypeReview:
		if count == 1 {
			return "1 new review"
		}
		return fmt.Sprintf("%d new reviews", count)
	case pullrequest.ActivityTypeComment:
		if count == 1 {
			return "1 new comment"
		}
		return fmt.Sprintf("%d new comments", count)
	case pullrequest.ActivityTypeReaction:
		if count == 1 {
			return "1 new reaction"
		}
		return fmt.Sprintf("%d new reactions", count)
	case pullrequest.ActivityTypeCommit:
		if count == 1 {
			return "1 new commit"
		}
		return fmt.Sprintf("%d new commits", count)
	default:
		if count == 1 {
			return "1 new activity"
		}
		return fmt.Sprintf("%d new activities", count)
	}
}

// SupportsClickActions returns false for the generic desktop adapter
func (a *Adapter) SupportsClickActions() bool {
	return false
}

// selectIcon selects the appropriate icon based on system theme
func (a *Adapter) selectIcon() []byte {
	theme := a.themeProvider.GetSystemTheme()
	if theme == "dark" {
		return assets.GitPullRequestIcon
	}
	return assets.GitPullRequestLightIcon
}
