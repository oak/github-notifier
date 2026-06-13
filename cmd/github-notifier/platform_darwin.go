//go:build darwin

package main

import (
	"github.com/rs/zerolog/log"

	"github.com/oak/github-notifier/application/port"
	"github.com/oak/github-notifier/infrastructure/notification/desktop"
	"github.com/oak/github-notifier/infrastructure/notification/macos"
	macosun "github.com/oak/github-notifier/infrastructure/notification/macos/un"
	"github.com/oak/github-notifier/infrastructure/ui"
)

func createDarwinNotifier(app *App, themeProvider *ui.SystemThemeProvider) port.NotificationPort {
	// Prefer native UNUserNotificationCenter (click-to-open, no external tools).
	// Only works when launched from a .app bundle; falls back to terminal-notifier.
	if unAdapter := macosun.NewAdapter(); unAdapter != nil {
		log.Info().Msg("Using native UNUserNotificationCenter adapter")
		return unAdapter
	}
	adapter, err := macos.NewAdapter(themeProvider, app.cfg.MacOSNotificationSender)
	if err != nil {
		log.Warn().Err(err).Msg("Falling back to desktop notifications")
		return desktop.NewAdapter(themeProvider)
	}
	return adapter
}
