package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/slack"
	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Adapter implements port.NotificationPort using Slack
type Adapter struct {
	notifier *notify.Notify
}

// NewAdapter creates a new Slack notification adapter
// The adapter sends direct messages to the authenticated user
func NewAdapter(oauthToken string) (port.NotificationPort, error) {
	// Create Slack service with OAuth token
	slackService := slack.New(oauthToken)

	// The notify library will use the token to send messages
	// We don't need to call AuthTest - the library handles authentication
	// For DMs to self, we can use a special channel ID or let the service determine it

	// Create notifier
	notifier := notify.New()
	notifier.UseServices(slackService)

	return &Adapter{
		notifier: notifier,
	}, nil
}

// NotifyPullRequests sends grouped notifications for pull requests
func (a *Adapter) NotifyPullRequests(notifications []*port.PRNotificationData) error {
	if len(notifications) == 0 {
		return nil
	}

	ctx := context.Background()

	// Send one message per PR
	for _, prNotif := range notifications {
		title, message := a.buildSlackMessage(prNotif)

		if err := a.notifier.Send(ctx, title, message); err != nil {
			log.Error().Msgf("Failed to send Slack notification: %v", err)
			// Continue sending other notifications
		} else {
			log.Info().Msgf("Sent Slack notification for PR #%d", prNotif.PullRequest.Number())
		}
	}

	return nil
}

// buildSlackMessage creates a Slack-formatted message for a PR notification
func (a *Adapter) buildSlackMessage(prNotif *port.PRNotificationData) (string, string) {
	pr := prNotif.PullRequest
	repoInfo := pr.Repository()

	// Build title
	var title string
	if prNotif.IsNew {
		title = fmt.Sprintf("🆕 New PR #%d", pr.Number())
	} else if prNotif.PipelineChange != nil {
		s := prNotif.PipelineChange.NewStatus
		title = fmt.Sprintf("%s PR #%d – Pipeline %s", s.Emoji(), pr.Number(), s.Label())
	} else if len(prNotif.ReviewChanges) > 0 {
		title = fmt.Sprintf("🔔 PR #%d Review", pr.Number())
	} else {
		title = fmt.Sprintf("🔔 PR #%d Activity", pr.Number())
	}

	// Build message
	var parts []string

	// Add PR link and title
	parts = append(parts, fmt.Sprintf("*<%s|%s #%d>*",
		pr.URL(),
		repoInfo.NameWithOwner(),
		pr.Number()))
	parts = append(parts, pr.Title())

	// Add pipeline status right after the PR title so it is immediately visible
	if prNotif.PipelineChange != nil {
		s := prNotif.PipelineChange.NewStatus
		parts = append(parts, fmt.Sprintf("*Pipeline:* %s %s", s.Emoji(), s.Label()))
	}

	// Add "NEW" indicator
	if prNotif.IsNew {
		parts = append(parts, "_Needs review_")
	}

	// Add activities
	if len(prNotif.Activities) > 0 {
		parts = append(parts, "") // Blank line
		activityLines := []string{"*Activity:*"}

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
					activityLines = append(activityLines, fmt.Sprintf("• %s", label))
				}
			}
		}

		parts = append(parts, strings.Join(activityLines, "\n"))
	}

	// Add status changes
	if len(prNotif.StatusChanges) > 0 {
		for _, statusChange := range prNotif.StatusChanges {
			if statusChange.EventType == pullrequest.StatusChangeMerged {
				parts = append(parts, "✅ *Merged*")
			} else if statusChange.EventType == pullrequest.StatusChangeClosed {
				parts = append(parts, "❌ *Closed*")
			}
		}
	}

	// Add review state changes
	if len(prNotif.ReviewChanges) > 0 {
		parts = append(parts, "") // Blank line
		reviewLines := []string{"*Reviews:*"}
		for _, reviewChange := range prNotif.ReviewChanges {
			reviewLines = append(reviewLines, fmt.Sprintf("• %s %s %s", reviewChange.State.Emoji(), reviewChange.Reviewer, reviewChange.State.Label()))
		}
		parts = append(parts, strings.Join(reviewLines, "\n"))
	}

	message := strings.Join(parts, "\n")
	return title, message
}

// getActivityLabel returns a formatted label for an activity
func (a *Adapter) getActivityLabel(actType pullrequest.ActivityType, count int) string {
	switch actType {
	case pullrequest.ActivityTypePush:
		if count == 1 {
			return "1 new commit pushed"
		}
		return fmt.Sprintf("%d new commits pushed", count)
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

// NotifyMessage sends a simple text notification via Slack
func (a *Adapter) NotifyMessage(title, message string) error {
	ctx := context.Background()
	return a.notifier.Send(ctx, title, message)
}

// SupportsClickActions returns false for Slack adapter (links are in message)
func (a *Adapter) SupportsClickActions() bool {
	return false
}
