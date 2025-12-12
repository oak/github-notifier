package app

import (
	"fmt"
	"log"
	"time"

	"github.com/oak3/github-notifier/application"
	"github.com/oak3/github-notifier/config"
	"github.com/oak3/github-notifier/domain"
	"github.com/oak3/github-notifier/ui"
)

// App represents the application state and logic
type App struct {
	config              *config.Config
	prService           application.PullRequestService
	notificationService application.NotificationService
	menuManager         *ui.MenuManager
	seenPRsForReview    map[string]bool
	seenPRsByUser       map[string]bool
	checkTicker         *time.Ticker
}

// NewApp creates a new application instance
func NewApp(
	cfg *config.Config,
	prService application.PullRequestService,
	notifService application.NotificationService,
	menuManager *ui.MenuManager,
) *App {
	return &App{
		config:              cfg,
		prService:           prService,
		notificationService: notifService,
		menuManager:         menuManager,
		seenPRsForReview:    make(map[string]bool),
		seenPRsByUser:       make(map[string]bool),
	}
}

// Start begins the PR checking loop
func (a *App) Start(checkInterval time.Duration) {
	// Initial check
	a.checkPRs()

	// Setup periodic checks
	a.checkTicker = time.NewTicker(checkInterval)
	go func() {
		for range a.checkTicker.C {
			a.checkPRs()
		}
	}()
}

// Stop halts the checking loop
func (a *App) Stop() {
	if a.checkTicker != nil {
		a.checkTicker.Stop()
	}
}

// checkPRs fetches PRs and updates the menu
func (a *App) checkPRs() {
	requestedReview, err := a.prService.FetchPRsRequestedReviews(a.config.GitHubToken)
	if err != nil {
		log.Printf("Error fetching Requested Review PRs: %v", err)
		return
	}

	usersPRs, err := a.prService.FetchUsersPRs(a.config.GitHubToken)
	if err != nil {
		log.Printf("Error fetching own PRs: %v", err)
		return
	}

	// Sort PRs before updating menu
	ui.SortPRsByCreatedAt(requestedReview)
	ui.SortPRsByCreatedAt(usersPRs)

	// Update menu
	a.menuManager.BuildMenu(requestedReview, usersPRs)

	// Check for new PRs and send notifications
	a.checkForNewPRs(requestedReview, usersPRs)
}

// checkForNewPRs identifies new PRs and sends notifications
func (a *App) checkForNewPRs(requestedReview, usersPRs []domain.PullRequest) {
	newPRsForReview := a.identifyNewPRs(requestedReview, a.seenPRsForReview)
	if len(newPRsForReview) > 0 {
		a.sendNotification("New PRs needing review", newPRsForReview)
	}

	newPRsByUser := a.identifyNewPRs(usersPRs, a.seenPRsByUser)
	if len(newPRsByUser) > 0 {
		a.sendNotification("New PRs by you", newPRsByUser)
	}
}

// identifyNewPRs identifies which PRs are new
func (a *App) identifyNewPRs(prs []domain.PullRequest, seen map[string]bool) []string {
	var newPRs []string
	for _, pr := range prs {
		if !seen[pr.URL] {
			seen[pr.URL] = true
			newPRs = append(newPRs, fmt.Sprintf("%s #%d", pr.Repository.NameWithOwner, pr.Number))
		}
	}
	return newPRs
}

// sendNotification sends a notification with PR information
func (a *App) sendNotification(title string, prs []string) {
	message := fmt.Sprintf("%s: %d", title, len(prs))
	for _, pr := range prs {
		message += "\n" + pr
	}
	err := a.notificationService.Notify("GitHub Notifier", message)
	if err != nil {
		log.Printf("Error sending notification: %v", err)
	}
}
