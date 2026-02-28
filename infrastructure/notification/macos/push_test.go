//go:build darwin

package macos

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestPushWithSender verifies that sending a notification via the system
// terminal-notifier works with various sender values. Run manually on macOS:
//
//	go test -v -run TestPushWithSender ./infrastructure/notification/macos/
func TestPushWithSender(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping in CI: no NotificationCenter instance available in headless runners")
	}
	path, err := exec.LookPath("terminal-notifier")
	if err != nil {
		t.Skip("terminal-notifier not found in PATH; install with: brew install terminal-notifier")
	}
	t.Logf("terminal-notifier: %s", path)

	senders := []struct {
		name   string
		sender string
	}{
		{"no sender", ""},
		{"com.apple.Terminal", "com.apple.Terminal"},
		{"com.apple.Safari", "com.apple.Safari"},
	}

	for _, tc := range senders {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &Adapter{binaryPath: path, sender: tc.sender}
			if err := adapter.push("github-notifier test", fmt.Sprintf("test message (%s)", tc.name), ""); err != nil {
				t.Errorf("push() with sender %q failed: %v", tc.sender, err)
			}
		})
	}
}
