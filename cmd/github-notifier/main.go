package main

import (
	"log"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/config"
	"github.com/oak3/github-notifier/domain/tracking"
	"github.com/oak3/github-notifier/infrastructure/github"
	"github.com/oak3/github-notifier/infrastructure/notification"
	"github.com/oak3/github-notifier/infrastructure/notification/desktop"
	"github.com/oak3/github-notifier/infrastructure/notification/slack"
	"github.com/oak3/github-notifier/infrastructure/persistence/memory"
	"github.com/oak3/github-notifier/infrastructure/ui"
)

// Application orchestrates the startup and lifecycle
type Application struct {
	cfg                      *config.Config
	checkPullRequestsUseCase *usecase.CheckPullRequestsUseCase
	menuAdapter              *ui.MenuAdapter
	checkTicker              *time.Ticker
	wg                       sync.WaitGroup // Track goroutines for clean shutdown
}

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	if !cfg.IsValid() {
		log.Fatal("GitHub token not configured. Set GITHUB_TOKEN environment variable.")
	}

	// Initialize infrastructure adapters
	githubAdapter := github.NewAdapter(cfg.GitHubToken)
	seenRepo := memory.NewSeenPullRequestRepository()
	trackingService := tracking.NewTrackingService(seenRepo)
	themeProvider := ui.NewSystemThemeProvider()

	// Setup notification adapters (desktop + optional Slack)
	var notificationAdapter port.NotificationPort
	desktopAdapter := desktop.NewAdapter(themeProvider)

	if cfg.SlackOAuthToken != "" {
		log.Println("Slack OAuth token detected, enabling Slack notifications...")
		slackAdapter, err := slack.NewAdapter(cfg.SlackOAuthToken)
		if err != nil {
			log.Printf("Warning: Failed to initialize Slack adapter: %v. Continuing with desktop-only notifications.", err)
			notificationAdapter = desktopAdapter
		} else {
			log.Println("Slack notifications enabled successfully")
			notificationAdapter = notification.NewCompositeAdapter(desktopAdapter, slackAdapter)
		}
	} else {
		notificationAdapter = desktopAdapter
	}

	menuAdapter := ui.NewMenuAdapter(cfg, themeProvider)

	// Initialize use case
	checkPullRequestsUseCase := usecase.NewCheckPullRequestsUseCase(
		githubAdapter,
		trackingService,
		notificationAdapter,
		menuAdapter,
		cfg.EnableActivityTracking,
		cfg.IncludeDraftPRs,
		cfg.RecentPRThresholdHours,
		cfg.StalePRCheckIntervalMin,
	)

	// Create application
	app := &Application{
		cfg:                      cfg,
		checkPullRequestsUseCase: checkPullRequestsUseCase,
		menuAdapter:              menuAdapter,
	}

	// Start systray
	systray.Run(app.onReady, app.onExit)
}

func (app *Application) onReady() {
	systray.SetTooltip("GitHub PR Notifier")

	// Setup menu
	app.menuAdapter.Setup()

	// Initial check - mark all existing PRs as seen first to avoid notifications/asterisks on startup
	log.Println("Performing initial PR check...")
	if err := app.checkPullRequestsUseCase.ExecuteInitial(); err != nil {
		log.Printf("Error during initial check: %v", err)
	}

	// Setup periodic checks
	app.checkTicker = time.NewTicker(time.Duration(app.cfg.CheckInterval) * time.Minute)
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		for range app.checkTicker.C {
			log.Println("Checking for PR updates...")
			if err := app.checkPullRequestsUseCase.Execute(); err != nil {
				log.Printf("Error checking PRs: %v", err)
			}
		}
	}()
}

func (app *Application) onExit() {
	log.Println("Shutting down...")
	if app.checkTicker != nil {
		app.checkTicker.Stop()
	}
	app.menuAdapter.Shutdown()
	// Wait for all goroutines to complete
	log.Println("Waiting for background tasks to complete...")
	app.wg.Wait()
	log.Println("Shutdown complete")
}
