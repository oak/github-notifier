//go:build !darwin

package main

import (
	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/infrastructure/notification/desktop"
	"github.com/oak3/github-notifier/infrastructure/ui"
)

func createDarwinNotifier(app *App, themeProvider *ui.SystemThemeProvider) port.NotificationPort {
	// Fallback for non-Darwin platforms (should not be called)
	return desktop.NewAdapter(themeProvider)
}
