package slack

import (
	"context"
	"fmt"
	"log"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/slack"
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

// NotifyNewPullRequests sends a Slack message about new pull requests
func (a *Adapter) NotifyNewPullRequests(title string, prs []*pullrequest.PullRequest) error {
	if len(prs) == 0 {
		return nil
	}

	// Build message with Slack markdown formatting
	slackTitle := fmt.Sprintf("🔔 *%s*", title)
	message := fmt.Sprintf("%s (%d)\n\n", slackTitle, len(prs))

	for _, pr := range prs {
		// Use Repository() method directly from PullRequest
		repoInfo := pr.Repository()
		message += fmt.Sprintf("• <%s|%s #%d>: %s\n",
			pr.URL(),
			repoInfo.NameWithOwner(),
			pr.Number(),
			pr.Title(),
		)
	}

	// Send notification
	ctx := context.Background()
	if err := a.notifier.Send(ctx, slackTitle, message); err != nil {
		log.Printf("Failed to send Slack notification: %v", err)
		return fmt.Errorf("slack notification failed: %w", err)
	}

	log.Printf("Sent Slack notification: %s with %d PRs", title, len(prs))
	return nil
}
