package main

import (
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/oak3/github-notifier/app"
	"github.com/oak3/github-notifier/config"
	"github.com/oak3/github-notifier/infrastructure/notification/beeep"
	githubpr "github.com/oak3/github-notifier/infrastructure/pr/github"
	"github.com/oak3/github-notifier/ui"
)

var application *app.App

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	if !cfg.IsValid() {
		log.Fatal("GitHub token not configured. Set GITHUB_TOKEN environment variable.")
	}

	// Initialize services
	prService := githubpr.NewGitHubPullRequestService(*cfg)
	notificationService := beeep.NewBeeepNotificationService()
	menuManager := ui.NewMenuManager(cfg, openURL)

	// Create application
	application = app.NewApp(cfg, prService, notificationService, menuManager)

	// Start systray
	systray.Run(onReady, onExit)
}

func onReady() {
	icon, err := os.ReadFile("icon.png")
	if err != nil {
		log.Printf("Failed to load icon: %v", err)
		icon = []byte{} // fallback
	}
	systray.SetIcon(icon)
	systray.SetTitle("GitHub Notifier")
	systray.SetTooltip("GitHub PR Notifier")

	// Start the application
	application.Start()
}

func onExit() {
	application.Stop()
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		log.Printf("Unsupported OS: %s", runtime.GOOS)
		return
	}
	cmd.Run()
}
