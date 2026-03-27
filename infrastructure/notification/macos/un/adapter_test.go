//go:build darwin

package un

import (
	"os"
	"testing"
	"time"
)

// TestSendWithURL sends a test notification with a clickable URL.
// Requires the binary to be running inside a proper .app bundle, so this must
// be run as part of the full app — it is skipped in test binaries and CI.
//
//	go test -v -run TestSendWithURL ./infrastructure/notification/macos/un/
func TestSendWithURL(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping in CI: no NotificationCenter available in headless runners")
	}

	a := NewAdapter()
	if a == nil {
		t.Skip("skipping: UNUserNotificationCenter unavailable (binary must be launched from a .app bundle)")
	}

	a.send("test-url", "GitHub Notifier", "Click to open github.com", "https://github.com")

	// Give the run loop time to deliver the notification
	time.Sleep(2 * time.Second)
}
