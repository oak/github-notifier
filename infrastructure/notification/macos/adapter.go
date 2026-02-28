package macos

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Adapter implements the NotificationPort interface for macOS using the
// system-installed terminal-notifier binary (brew install terminal-notifier).
type Adapter struct {
	themeProvider ThemeProvider
	sender        string
	binaryPath    string
}

// ThemeProvider provides the current system theme
type ThemeProvider interface {
	GetSystemTheme() string
}

// NewAdapter creates a new macOS notification adapter.
// Returns an error if terminal-notifier is not found in PATH.
func NewAdapter(themeProvider ThemeProvider, sender string) (*Adapter, error) {
	path, err := exec.LookPath("terminal-notifier")
	if err != nil {
		return nil, errors.New("terminal-notifier not found in PATH; install it with: brew install terminal-notifier")
	}
	return &Adapter{
		themeProvider: themeProvider,
		sender:        sender,
		binaryPath:    path,
	}, nil
}

// NotifyPullRequests sends grouped notifications for pull requests
func (a *Adapter) NotifyPullRequests(notifications []*port.PRNotificationData) error {
	if len(notifications) == 0 {
		return nil
	}

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
	title := a.buildNotificationTitle(prNotif)
	message := a.buildNotificationMessage(prNotif)
	return a.push(title, message, pr.URL())
}

// push invokes terminal-notifier with the given title, message, open URL, and optional sender.
func (a *Adapter) push(title, message, openURL string) error {
	args := []string{
		"-title", title,
		"-message", message,
		"-sound", "default",
	}
	if openURL != "" {
		args = append(args, "-open", openURL)
	}
	if a.sender != "" {
		args = append(args, "-sender", a.sender)
	}

	out, err := exec.Command(a.binaryPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("terminal-notifier: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// buildNotificationTitle creates the title for a PR notification
func (a *Adapter) buildNotificationTitle(prNotif *port.PRNotificationData) string {
	pr := prNotif.PullRequest

	if prNotif.IsNew {
		return fmt.Sprintf("New PR #%d", pr.Number())
	}

	if prNotif.PipelineChange != nil {
		s := prNotif.PipelineChange.NewStatus
		return fmt.Sprintf("PR #%d Pipeline %s %s", pr.Number(), s.Label(), s.Emoji())
	}

	if len(prNotif.ReviewChanges) > 0 {
		return fmt.Sprintf("PR #%d Review", pr.Number())
	}

	return fmt.Sprintf("PR #%d Activity", pr.Number())
}

// buildNotificationMessage creates the message for a PR notification
func (a *Adapter) buildNotificationMessage(prNotif *port.PRNotificationData) string {
	pr := prNotif.PullRequest
	var parts []string

	parts = append(parts, pr.RepositoryName())
	parts = append(parts, pr.Title())

	if prNotif.IsNew {
		parts = append(parts, "🆕 Needs review")
	}

	// Add pipeline status right after the PR title so it is immediately visible
	if prNotif.PipelineChange != nil {
		s := prNotif.PipelineChange.NewStatus
		parts = append(parts, fmt.Sprintf("Pipeline: %s %s", s.Emoji(), s.Label()))
	}

	if len(prNotif.Activities) > 0 {
		activityLines := []string{}

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

	for _, statusChange := range prNotif.StatusChanges {
		if statusChange.EventType == pullrequest.StatusChangeMerged {
			parts = append(parts, "✅ Merged")
		} else if statusChange.EventType == pullrequest.StatusChangeClosed {
			parts = append(parts, "❌ Closed")
		}
	}

	for _, reviewChange := range prNotif.ReviewChanges {
		parts = append(parts, fmt.Sprintf("%s %s %s", reviewChange.State.Emoji(), reviewChange.Reviewer, reviewChange.State.Label()))
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

// NotifyMessage sends a simple text notification via macOS native notifications
func (a *Adapter) NotifyMessage(title, message string) error {
	return a.push(title, message, "")
}

// SupportsClickActions returns true for macOS adapter
func (a *Adapter) SupportsClickActions() bool {
	return true
}
