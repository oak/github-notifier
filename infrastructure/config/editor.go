package config

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenInEditor opens the config file in the user's default text editor.
// It uses platform-specific commands: xdg-open (Linux), open -t (macOS), notepad (Windows).
func OpenInEditor(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "darwin":
		cmd = exec.Command("open", "-t", path)
	case "windows":
		cmd = exec.Command("notepad", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// editorCommand returns the command and args that would be used to open a file.
// Exported for testing only via the test file.
func editorCommand(path string) (string, []string) {
	switch runtime.GOOS {
	case "linux":
		return "xdg-open", []string{path}
	case "darwin":
		return "open", []string{"-t", path}
	case "windows":
		return "notepad", []string{path}
	default:
		return "", nil
	}
}
