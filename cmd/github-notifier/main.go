package main

import (
	"log"
	"time"

	"github.com/getlantern/systray"
	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/config"
	"github.com/oak3/github-notifier/domain/tracking"
	"github.com/oak3/github-notifier/infrastructure/github"
	"github.com/oak3/github-notifier/infrastructure/notification"
	"github.com/oak3/github-notifier/infrastructure/persistence/memory"
	"github.com/oak3/github-notifier/infrastructure/ui"
)

// Application orchestrates the startup and lifecycle
type Application struct {
	cfg                      *config.Config
	checkPullRequestsUseCase *usecase.CheckPullRequestsUseCase
	menuAdapter              *ui.MenuAdapter
	checkTicker              *time.Ticker
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
	themeProvider := notification.NewSystemThemeProvider()
	notificationAdapter := notification.NewAdapter(themeProvider)
	menuAdapter := ui.NewMenuAdapter(cfg, themeProvider)

	// Initialize use case
	checkPullRequestsUseCase := usecase.NewCheckPullRequestsUseCase(
		githubAdapter,
		trackingService,
		notificationAdapter,
		menuAdapter,
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
	systray.SetTitle("GitHub Notifier")
	systray.SetTooltip("GitHub PR Notifier")

	// Setup menu
	app.menuAdapter.Setup()

	// Initial check
	log.Println("Performing initial PR check...")
	if err := app.checkPullRequestsUseCase.Execute(); err != nil {
		log.Printf("Error during initial check: %v", err)
	}

	// Setup periodic checks
	app.checkTicker = time.NewTicker(time.Duration(app.cfg.CheckInterval) * time.Minute)
	go func() {
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
}
