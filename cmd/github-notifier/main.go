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
	jsonrepo "github.com/oak3/github-notifier/infrastructure/persistence/json"
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

	log.Info().Str("config_file", cfg.ConfigFilePath).Msg("Starting GitHub PR Notifier...")

	// Create application with context
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
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
	if app.cfg.IsValid() {
		app.startWithConfig(app.cfg)
		return
	}

	// Token not set — enter waiting mode
	log.Warn().
		Str("config_file", app.cfg.ConfigFilePath).
		Msg("GitHub token not configured — waiting for user to set it")

	app.enterWaitingMode()
}

// enterWaitingMode sets up a minimal systray, notifies the user, opens the
// config file in their editor, and watches for a valid config to appear.
func (app *App) enterWaitingMode() {
	// We need a MenuAdapter just for the waiting state UI
	themeProvider := ui.NewSystemThemeProvider()
	app.menuAdapter = ui.NewMenuAdapter(1, 1, themeProvider, "")
	app.menuAdapter.SetupWaitingState(app.cfg.ConfigFilePath)

	// Send a desktop notification to inform the user via the proper port
	desktopNotifier := app.createDesktopNotifier(themeProvider)
	if err := desktopNotifier.NotifyMessage(
		"GitHub Notifier — Setup Required",
		"GitHub token not configured. Opening config file...",
	); err != nil {
		log.Warn().Err(err).Msg("Failed to send setup notification")
	}

	// Open the config file in the default editor
	if err := config.OpenInEditor(app.cfg.ConfigFilePath); err != nil {
		log.Warn().Err(err).Msg("Failed to open config file in editor")
	}

	// Start watching the config file for a valid token
	validCfgCh := config.WatchForValidConfig(app.ctx, app.cfg.ConfigFilePath)

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		select {
		case cfg, ok := <-validCfgCh:
			if !ok || cfg == nil {
				return
			}
			log.Info().Msg("Valid configuration detected — initializing application")
			app.cfg = cfg
			app.startWithConfig(cfg)
		case <-app.ctx.Done():
			return
		}
	}()
}

// createDesktopNotifier creates an OS-appropriate desktop notification adapter.
// Used both for the waiting-mode setup notification and the normal runtime.
func (app *App) createDesktopNotifier(themeProvider *ui.SystemThemeProvider) port.NotificationPort {
	switch runtime.GOOS {
	case "darwin":
		adapter, err := macos.NewAdapter(themeProvider, app.cfg.MacOSNotificationSender)
		if err != nil {
			log.Warn().Err(err).Msg("Falling back to desktop notifications")
			return desktop.NewAdapter(themeProvider)
		}
		return adapter
	case "linux":
		return linux.NewAdapter(themeProvider)
	default:
		return desktop.NewAdapter(themeProvider)
	}
}

// startWithConfig initializes all infrastructure and starts the polling loop.
// It can be called either at startup (happy path) or after the config watcher
// detects a valid config (waiting path).
func (app *App) startWithConfig(cfg *config.Config) {
	// Initialize infrastructure adapters
	githubAdapter := github.NewAdapter(cfg.GitHubToken)
	stateRepo := jsonrepo.NewStateRepository(cfg.StateFilePath())
	trackingService := pullrequest.NewTrackingService(stateRepo)
	themeProvider := ui.NewSystemThemeProvider()

	// Setup notification adapters (OS-specific desktop + optional Slack)
	var notificationAdapter port.NotificationPort
	desktopAdapter := app.createDesktopNotifier(themeProvider)

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

	// Clear waiting-state menu items if transitioning from waiting mode
	if app.menuAdapter != nil {
		app.menuAdapter.ClearWaitingState()
	}

	// Create menu adapter (replaces waiting-state adapter if present)
	app.menuAdapter = ui.NewMenuAdapter(cfg.MaxNumberOfRepos, cfg.MaxNumberOfPRs, themeProvider, githubAdapter.AuthenticatedUser())

	systray.SetTooltip("GitHub PR Notifier")
	app.menuAdapter.Setup()

	// Initialize domain services
	prFilter := pullrequest.NewDraftFilter(cfg.IncludeDraftPRs)
	activityScheduler := pullrequest.NewActivityCheckScheduler(
		cfg.RecentPRThresholdHours,
		cfg.StalePRCheckIntervalMin,
	)

	// Initialize event infrastructure
	eventBus := events.NewInMemoryEventBus()

	// Register event handlers
	notificationHandler := events.NewNotificationEventHandler(notificationAdapter, githubAdapter.AuthenticatedUser())
	trackingHandler := events.NewTrackingEventHandler(trackingService)

	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, notificationHandler)
	eventBus.Subscribe(pullrequest.EventReviewStateChanged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventMerged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventClosed, notificationHandler)
	eventBus.Subscribe(pullrequest.EventPipelineStatusChanged, notificationHandler)
	eventBus.Subscribe(pullrequest.EventNewPullRequestDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventActivityDetected, trackingHandler)
	eventBus.Subscribe(pullrequest.EventReviewStateChanged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventMerged, trackingHandler)
	eventBus.Subscribe(pullrequest.EventClosed, trackingHandler)
	eventBus.Subscribe(pullrequest.EventPipelineStatusChanged, trackingHandler)

	// Initialize use cases
	initializeUseCase := usecase.NewInitializeFirstCheckUseCase(
		githubAdapter,
		trackingService,
		prFilter,
		app.menuAdapter,
	)

	checkNewPRsUseCase := usecase.NewCheckNewPullRequestsUseCase(
		githubAdapter,
		trackingService,
		prFilter,
		eventBus,
	)

	detectClosedPRsUseCase := usecase.NewDetectClosedPullRequestsUseCase(
		githubAdapter,
		stateRepo,
		eventBus,
	)

	trackActivityUseCase := usecase.NewTrackPullRequestActivityUseCase(
		githubAdapter,
		stateRepo,
		activityScheduler,
		trackingService,
		eventBus,
		githubAdapter.AuthenticatedUser(),
	)

	updateDisplayUseCase := usecase.NewUpdatePullRequestDisplayUseCase(
		app.menuAdapter,
		trackingService,
	)

	// Create orchestrator
	app.orchestrator = application.NewPullRequestOrchestrator(
		initializeUseCase,
		checkNewPRsUseCase,
		detectClosedPRsUseCase,
		trackActivityUseCase,
		updateDisplayUseCase,
		cfg.EnableActivityTracking,
	)

	// Initial check
	if err := app.orchestrator.ExecuteInitialCheck(app.ctx); err != nil {
		log.Error().Err(err).Msg("Error during initial check")
	}

	// Setup periodic checks with context cancellation
	app.checkTicker = time.NewTicker(time.Duration(cfg.CheckInterval) * time.Minute)
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
	if app.menuAdapter != nil {
		app.menuAdapter.Shutdown()
	}

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
