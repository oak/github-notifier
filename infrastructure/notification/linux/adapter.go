package linux

import (
	"fmt"
	"strings"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"
	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Adapter implements the NotificationPort interface for Linux using D-Bus
type Adapter struct {
	themeProvider ThemeProvider
	conn          *dbus.Conn
	notifier      notify.Notifier
}

// ThemeProvider provides the current system theme
type ThemeProvider interface {
	GetSystemTheme() string
}

// NewAdapter creates a new Linux notification adapter using D-Bus
func NewAdapter(themeProvider ThemeProvider) *Adapter {
	// Connect to session bus
	conn, err := dbus.SessionBus()
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to D-Bus session bus, notifications may not work")
		return &Adapter{
			themeProvider: themeProvider,
		}
	}

	notifier, err := notify.New(conn)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create D-Bus notifier, notifications may not work")
		return &Adapter{
			themeProvider: themeProvider,
			conn:          conn,
		}
	}

	return &Adapter{
		themeProvider: themeProvider,
		conn:          conn,
		notifier:      notifier,
	}
}

// NotifyPullRequests sends grouped notifications for pull requests
func (a *Adapter) NotifyPullRequests(notifications []*port.PRNotificationData) error {
	if len(notifications) == 0 {
		return nil
	}

	if a.notifier == nil {
		log.Warn().Msg("D-Bus notifier not initialized, skipping notification")
		return nil
	}

	// Send one notification per PR
	for _, prNotif := range notifications {
		if err := a.sendPRNotification(prNotif); err != nil {
			log.Error().Err(err).Msg("Error sending PR notification")
			// Continue sending other notifications even if one fails
		}
	}

	return nil
}

// sendPRNotification sends a single PR notification with all its activities
func (a *Adapter) sendPRNotification(prNotif *port.PRNotificationData) error {
	pr := prNotif.PullRequest

	// Build the title
	title := a.buildNotificationTitle(prNotif)

	// Build the message body
	body := a.buildNotificationBody(prNotif)

	notification := notify.Notification{
		AppName:       "GitHub Notifier",
		Summary:       title,
		Body:          body,
		ExpireTimeout: 5000, // 5 seconds
	}

	// Add click action to open the PR
	urlToOpen := pr.URL()
	notification.Actions = []notify.Action{
		{
			Key:   "default",
			Label: "Open PR",
		},
	}

	// Set up action handler in a goroutine to listen for clicks
	go a.handleNotificationActions(urlToOpen)

	// Send notification
	_, err := a.notifier.SendNotification(notification)
	if err != nil {
		log.Error().Err(err).Msg("Error sending Linux notification")
		return err
	}

	return nil
}

// buildNotificationTitle creates the title for a PR notification
func (a *Adapter) buildNotificationTitle(prNotif *port.PRNotificationData) string {
	pr := prNotif.PullRequest

	if prNotif.IsNew {
		return "New PR"
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

// buildNotificationBody creates the body for a PR notification
func (a *Adapter) buildNotificationBody(prNotif *port.PRNotificationData) string {
	pr := prNotif.PullRequest
	var parts []string

	// Add PR info
	parts = append(parts, fmt.Sprintf("%s #%d", pr.RepositoryName(), pr.Number()))
	parts = append(parts, pr.Title())

	// Add "NEW" indicator if this is a new PR
	if prNotif.IsNew {
		parts = append(parts, "🆕 Needs review")
	}

	// Add pipeline status right after the PR title so it is immediately visible
	if prNotif.PipelineChange != nil {
		s := prNotif.PipelineChange.NewStatus
		parts = append(parts, fmt.Sprintf("Pipeline: %s %s", s.Emoji(), s.Label()))
	}

	// Add activities
	if len(prNotif.Activities) > 0 {
		activityLines := []string{}

		// Sort by priority: push > review > comment > reaction
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
			parts = append(parts, "\n"+strings.Join(activityLines, "\n"))
		}
	}

	// Add status changes
	for _, statusChange := range prNotif.StatusChanges {
		if statusChange.EventType == pullrequest.StatusChangeMerged {
			parts = append(parts, "✅ Merged")
		} else if statusChange.EventType == pullrequest.StatusChangeClosed {
			parts = append(parts, "❌ Closed")
		}
	}

	// Add review state changes
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

// handleNotificationActions listens for notification action signals and opens URLs
func (a *Adapter) handleNotificationActions(url string) {
	// Listen for action invoked signals
	// This is a simplified version - in production you'd want to match specific notification IDs
	// and handle cleanup properly
	signals := make(chan *dbus.Signal, 10)
	a.conn.Signal(signals)

	// Match both ActionInvoked (for action buttons) and ActivationToken (for notification body clicks)
	a.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.Notifications',member='ActionInvoked'")
	a.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.Notifications',member='ActivationToken'")

	// Listen for the action - either button click or notification body click
	sig := <-signals

	// Handle ActivationToken (notification body clicked) - common on KDE/KWin
	if sig.Name == "org.freedesktop.Notifications.ActivationToken" {
		log.Debug().Str("signal", sig.Name).Msg("Notification clicked, opening URL")
		if err := a.openURL(url); err != nil {
			log.Error().Err(err).Msg("Failed to open PR URL")
		}
		return
	}

	// Handle ActionInvoked (action button clicked)
	if sig.Name == "org.freedesktop.Notifications.ActionInvoked" && len(sig.Body) >= 2 {
		actionKey, ok := sig.Body[1].(string)
		if ok && actionKey == "default" {
			log.Debug().Str("action", actionKey).Msg("Notification action invoked, opening URL")
			if err := a.openURL(url); err != nil {
				log.Error().Err(err).Msg("Failed to open PR URL")
			}
		}
	}
}

// openURL opens a URL in the default browser
func (a *Adapter) openURL(url string) error {
	// Use xdg-open on Linux to open URLs in the default browser
	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	call := obj.Call("org.freedesktop.portal.OpenURI.OpenURI", 0, "", url, map[string]dbus.Variant{})

	if call.Err != nil {
		// Fallback to xdg-open if portal doesn't work
		log.Debug().Msg("Portal failed, attempting xdg-open fallback")
		// This would require exec.Command, but keeping D-Bus for now
		return call.Err
	}

	log.Info().Str("url", url).Msg("Opened PR URL")
	return nil
}

// NotifyMessage sends a simple text notification via D-Bus
func (a *Adapter) NotifyMessage(title, message string) error {
	if a.notifier == nil {
		log.Warn().Msg("D-Bus notifier not initialized, skipping notification")
		return nil
	}

	notification := notify.Notification{
		AppName:       "GitHub Notifier",
		Summary:       title,
		Body:          message,
		ExpireTimeout: 10000, // 10 seconds for setup messages
	}

	_, err := a.notifier.SendNotification(notification)
	return err
}

// SupportsClickActions returns true for Linux adapter
func (a *Adapter) SupportsClickActions() bool {
	return a.notifier != nil
}
