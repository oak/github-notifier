package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/oak3/github-notifier/application"
	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/config"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/events"
	"github.com/oak3/github-notifier/infrastructure/github"
	"github.com/oak3/github-notifier/infrastructure/notification"
	"github.com/oak3/github-notifier/infrastructure/notification/desktop"
	"github.com/oak3/github-notifier/infrastructure/notification/slack"
	"github.com/oak3/github-notifier/infrastructure/persistence/memory"
	"github.com/oak3/github-notifier/infrastructure/ui"
)

// App orchestrates the startup and lifecycle
type App struct {
	cfg          *config.Config
	orchestrator *application.PullRequestOrchestrator
	menuAdapter  *ui.MenuAdapter
	checkTicker  *time.Ticker
	ctx          context.Context    // Application context
	cancel       context.CancelFunc // Cancel function for graceful shutdown
	wg           sync.WaitGroup     // Track goroutines for clean shutdown
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
	trackingService := pullrequest.NewTrackingService(seenRepo)
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

	// Initialize domain services
	prFilter := pullrequest.NewPRFilter(cfg.IncludeDraftPRs)
	prClassifier := pullrequest.NewPRClassifier()
	activityScheduler := pullrequest.NewActivityCheckScheduler(
		cfg.RecentPRThresholdHours,
		cfg.StalePRCheckIntervalMin,
	)

	// Initialize event infrastructure
	eventBus := events.NewInMemoryEventBus()

	// Register event handlers
	notificationHandler := events.NewNotificationEventHandler(notificationAdapter)
	trackingHandler := events.NewTrackingEventHandler(trackingService)

	eventBus.Subscribe("NewPullRequestDetected", notificationHandler)
	eventBus.Subscribe("PullRequestActivityDetected", notificationHandler)
	eventBus.Subscribe("NewPullRequestDetected", trackingHandler)
	eventBus.Subscribe("PullRequestActivityDetected", trackingHandler)

	// Initialize use cases
	initializeUseCase := usecase.NewInitializeFirstCheckUseCase(
		githubAdapter,
		trackingService,
		prFilter,
		menuAdapter,
	)

	checkNewPRsUseCase := usecase.NewCheckNewPullRequestsUseCase(
		githubAdapter,
		trackingService,
		prFilter,
		prClassifier,
		eventBus,
	)

	trackActivityUseCase := usecase.NewTrackPullRequestActivityUseCase(
		githubAdapter,
		activityScheduler,
		trackingService,
		eventBus,
	)

	updateDisplayUseCase := usecase.NewUpdatePullRequestDisplayUseCase(
		menuAdapter,
		trackingService,
	)

	// Create orchestrator
	orchestrator := application.NewPullRequestOrchestrator(
		initializeUseCase,
		checkNewPRsUseCase,
		trackActivityUseCase,
		updateDisplayUseCase,
		cfg.EnableActivityTracking,
	)

	// Create application with context
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		cfg:          cfg,
		orchestrator: orchestrator,
		menuAdapter:  menuAdapter,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Start signal handler in background
	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
		systray.Quit()
	}()

	// Start systray (blocking call)
	log.Println("Starting GitHub PR Notifier...")
	systray.Run(app.onReady, app.onExit)
	log.Println("Application terminated")
}

func (app *App) onReady() {
	systray.SetTooltip("GitHub PR Notifier")

	// Setup menu
	app.menuAdapter.Setup()

	// Initial check
	if err := app.orchestrator.ExecuteInitialCheck(app.ctx); err != nil {
		log.Printf("Error during initial check: %v", err)
	}

	// Setup periodic checks with context cancellation
	app.checkTicker = time.NewTicker(time.Duration(app.cfg.CheckInterval) * time.Minute)
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		for {
			select {
			case <-app.ctx.Done():
				log.Println("Check goroutine received cancellation signal")
				return
			case <-app.checkTicker.C:
				log.Println("Checking for PR updates...")
				if err := app.orchestrator.ExecuteRegularCheck(app.ctx); err != nil {
					log.Printf("Error checking PRs: %v", err)
				}
			}
		}
	}()
}

func (app *App) onExit() {
	log.Println("Shutting down...")

	// Cancel context to stop goroutines
	app.cancel()

	// Stop ticker
	if app.checkTicker != nil {
		app.checkTicker.Stop()
	}

	// Shutdown menu adapter
	app.menuAdapter.Shutdown()

	// Wait for all goroutines to complete with timeout
	log.Println("Waiting for background tasks to complete...")
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Shutdown complete")
	case <-time.After(5 * time.Second):
		log.Println("Shutdown timeout - forcing exit")
	}
}
