package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/oak3/github-notifier/domain"
)

// groupPRsByRepository groups PRs by their repository
func groupPRsByRepository(prs []domain.PullRequest) map[string][]domain.PullRequest {
	grouped := make(map[string][]domain.PullRequest)
	for _, pr := range prs {
		grouped[pr.Repository.NameWithOwner] = append(grouped[pr.Repository.NameWithOwner], pr)
	}
	return grouped
}

// SortPRsByCreatedAt sorts PRs by creation time (oldest to newest)
func SortPRsByCreatedAt(prs []domain.PullRequest) {
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].CreatedAt.Before(prs[j].CreatedAt)
	})
}

// formatPRTitle returns a formatted PR title with age information
func formatPRTitle(pr domain.PullRequest) string {
	return fmt.Sprintf("[%s] [#%d] %s", formatTimeAgo(pr.CreatedAt), pr.Number, pr.Title)
}

// formatTimeAgo returns a human-readable time difference
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	hours := duration.Hours()
	days := hours / 24
	weeks := int(days / 7)

	if weeks > 0 {
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if days >= 2 {
		return fmt.Sprintf("%d days ago", int(days))
	} else {
		return fmt.Sprintf("%d hours ago", int(hours))
	}
}

// GetSystemTheme detects the system theme (dark or light) across platforms
func GetSystemTheme() string {
	switch runtime.GOOS {
	case "darwin":
		return getmacOSTheme()
	case "linux":
		return getLinuxTheme()
	case "windows":
		return getWindowsTheme()
	default:
		return "light" // fallback
	}
}

// getmacOSTheme detects macOS appearance settings
func getmacOSTheme() string {
	cmd := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle")
	output, err := cmd.Output()
	if err != nil {
		return "light" // defaults to light on error
	}
	if strings.Contains(string(output), "Dark") {
		return "dark"
	}
	return "light"
}

// getLinuxTheme detects Linux theme via freedesktop.org portal
func getLinuxTheme() string {
	// Use freedesktop.org portal settings (supports most modern Linux desktops)
	// Returns: 0 = no preference, 1 = dark, 2 = light
	cmd := exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.portal.Desktop",
		"--object-path", "/org/freedesktop/portal/desktop",
		"--method", "org.freedesktop.portal.Settings.Read",
		"org.freedesktop.appearance", "color-scheme")
	output, err := cmd.Output()
	if err == nil {
		outputStr := strings.TrimSpace(string(output))
		// Parse the output to find the color-scheme value
		if strings.Contains(outputStr, "int32 1") {
			return "dark"
		} else if strings.Contains(outputStr, "int32 2") {
			return "light"
		}
	}

	return "light"
}

// getWindowsTheme detects Windows theme from registry
func getWindowsTheme() string {
	// Check Windows Registry for AppsUseLightTheme setting
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize' -Name 'AppsUseLightTheme' -ErrorAction SilentlyContinue).AppsUseLightTheme")
	output, err := cmd.Output()
	if err == nil {
		if strings.Contains(string(output), "0") {
			return "dark"
		}
	}
	return "light"
}
