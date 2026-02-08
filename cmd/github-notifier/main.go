package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application"
	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/application/usecase"
	"github.com/oak3/github-notifier/config"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/events"
	"github.com/oak3/github-notifier/infrastructure/github"
	"github.com/oak3/github-notifier/infrastructure/notification"
	"github.com/oak3/github-notifier/infrastructure/notification/desktop"
	"github.com/oak3/github-notifier/infrastructure/notification/linux"
	"github.com/oak3/github-notifier/infrastructure/notification/macos"
	"github.com/oak3/github-notifier/infrastructure/notification/slack"
	"github.com/oak3/github-notifier/infrastructure/persistence/memory"
	"github.com/oak3/github-notifier/infrastructure/ui"
	"github.com/oak3/github-notifier/internal/logger"
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
	// Initialize logger
	logger.Initialize()

	// Load configuration
	cfg := config.LoadConfig()
	if !cfg.IsValid() {
		log.Fatal().Msg("GitHub token not configured. Set GITHUB_TOKEN environment variable.")
	}

	log.Info().Msg("Starting GitHub PR Notifier...")

	// Initialize infrastructure adapters
	githubAdapter := github.NewAdapter(cfg.GitHubToken)
	seenRepo := memory.NewSeenPullRequestRepository()
	trackingService := pullrequest.NewTrackingService(seenRepo)
	themeProvider := ui.NewSystemThemeProvider()

	// Setup notification adapters (OS-specific desktop + optional Slack)
	var notificationAdapter port.NotificationPort
	var desktopAdapter port.NotificationPort

	// Use OS-specific adapter for better native support
	switch runtime.GOOS {
	case "darwin":
		log.Info().Msg("Using macOS native notifications with click action support")
		desktopAdapter = macos.NewAdapter(themeProvider)
	case "linux":
		log.Info().Msg("Using Linux native notifications with click action support")
		desktopAdapter = linux.NewAdapter(themeProvider)
	default:
		log.Info().Msgf("Using generic desktop notifications for %s", runtime.GOOS)
		desktopAdapter = desktop.NewAdapter(themeProvider)
	}

	if desktopAdapter.SupportsClickActions() {
		log.Info().Msg("Click actions enabled - clicking notifications will open PRs")
	}

	if cfg.SlackOAuthToken != "" {
		log.Info().Msg("Slack OAuth token detected, enabling Slack notifications")
		slackAdapter, err := slack.NewAdapter(cfg.SlackOAuthToken)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("Failed to initialize Slack adapter. Continuing with desktop-only notifications")
			notificationAdapter = desktopAdapter
		} else {
			log.Info().Msg("Slack notifications enabled successfully")
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
	eventBus.Subscribe("ActivityDetected", notificationHandler)
	eventBus.Subscribe("Merged", notificationHandler)
	eventBus.Subscribe("Closed", notificationHandler)
	eventBus.Subscribe("NewPullRequestDetected", trackingHandler)
	eventBus.Subscribe("PullRequestActivityDetected", trackingHandler)
	eventBus.Subscribe("ActivityDetected", trackingHandler)
	eventBus.Subscribe("Merged", trackingHandler)
	eventBus.Subscribe("Closed", trackingHandler)

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
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		systray.Quit()
	}()

	// Start systray (blocking call)
	log.Info().Msg("Application starting")
	systray.Run(app.onReady, app.onExit)
	log.Info().Msg("Application terminated")
}

func (app *App) onReady() {
	systray.SetTooltip("GitHub PR Notifier")

	// Setup menu
	app.menuAdapter.Setup()

	// Initial check
	if err := app.orchestrator.ExecuteInitialCheck(app.ctx); err != nil {
		log.Error().Err(err).Msg("Error during initial check")
	}

	// Setup periodic checks with context cancellation
	app.checkTicker = time.NewTicker(time.Duration(app.cfg.CheckInterval) * time.Minute)
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		for {
			select {
			case <-app.ctx.Done():
				log.Debug().Msg("Check goroutine received cancellation signal")
				return
			case <-app.checkTicker.C:
				log.Info().Msg("Checking for PR updates")
				if err := app.orchestrator.ExecuteRegularCheck(app.ctx); err != nil {
					log.Error().Err(err).Msg("Error checking PRs")
				}
			}
		}
	}()
}

func (app *App) onExit() {
	log.Info().Msg("Shutting down")

	// Cancel context to stop goroutines
	app.cancel()

	// Stop ticker
	if app.checkTicker != nil {
		app.checkTicker.Stop()
	}

	// Shutdown menu adapter
	app.menuAdapter.Shutdown()

	// Wait for all goroutines to complete with timeout
	log.Info().Msg("Waiting for background tasks to complete")
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Shutdown complete")
	case <-time.After(5 * time.Second):
		log.Warn().Msg("Shutdown timeout - forcing exit")
	}
}
