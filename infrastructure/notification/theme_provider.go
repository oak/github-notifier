package notification

import (
	"os/exec"
	"runtime"
	"strings"
)

// SystemThemeProvider implements ThemeProvider using system detection
type SystemThemeProvider struct{}

// NewSystemThemeProvider creates a new system theme provider
func NewSystemThemeProvider() *SystemThemeProvider {
	return &SystemThemeProvider{}
}

// GetSystemTheme detects the system theme (dark or light) across platforms
func (p *SystemThemeProvider) GetSystemTheme() string {
	switch runtime.GOOS {
	case "darwin":
		return p.getMacOSTheme()
	case "linux":
		return p.getLinuxTheme()
	case "windows":
		return p.getWindowsTheme()
	default:
		return "light"
	}
}

// getMacOSTheme detects macOS appearance settings
func (p *SystemThemeProvider) getMacOSTheme() string {
	cmd := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle")
	output, err := cmd.Output()
	if err != nil {
		return "light"
	}
	if strings.Contains(string(output), "Dark") {
		return "dark"
	}
	return "light"
}

// getLinuxTheme detects Linux theme via freedesktop.org portal
func (p *SystemThemeProvider) getLinuxTheme() string {
	cmd := exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.portal.Desktop",
		"--object-path", "/org/freedesktop/portal/desktop",
		"--method", "org.freedesktop.portal.Settings.Read",
		"org.freedesktop.appearance", "color-scheme")
	output, err := cmd.Output()
	if err == nil {
		outputStr := strings.TrimSpace(string(output))
		if strings.Contains(outputStr, "int32 1") {
			return "dark"
		} else if strings.Contains(outputStr, "int32 2") {
			return "light"
		}
	}
	return "light"
}

// getWindowsTheme detects Windows theme from registry
func (p *SystemThemeProvider) getWindowsTheme() string {
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
