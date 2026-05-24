package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const DefaultIgnoreFileName = "ignore.yaml"

// DefaultIgnoreFilePath returns the default path for ignore.yaml, next to the config file.
func DefaultIgnoreFilePath(configFilePath string) string {
	dir := filepath.Dir(configFilePath)
	return filepath.Join(dir, DefaultIgnoreFileName)
}

// CreateDefaultIgnoreFile writes a self-documenting ignore.yaml if it does not exist.
func CreateDefaultIgnoreFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	return os.WriteFile(path, []byte(generateDefaultIgnoreContent()), 0600)
}

// generateDefaultIgnoreContent returns a documented ignore.yaml template.
func generateDefaultIgnoreContent() string {
	var b strings.Builder
	b.WriteString("# github-notifier ignore.yaml\n")
	b.WriteString("#\n")
	b.WriteString("# Suppress notifications for specific events, authors, or repositories.\n")
	b.WriteString("# Missing sections are ignored — no filtering applied.\n")
	b.WriteString("#\n")
	b.WriteString("# Supported event names:\n")
	b.WriteString("#   NewPullRequestDetected  ActivityDetected  Merged  Closed\n")
	b.WriteString("#   StatusChanged  ReviewStateChanged  PipelineStatusChanged\n")
	b.WriteString("#\n")
	b.WriteString("# 'except' entries use the format  event:detail, e.g.:\n")
	b.WriteString("#   PipelineStatusChanged:failed   ReviewStateChanged:changes_requested\n")
	b.WriteString("#\n")
	b.WriteString("# Structure:\n")
	b.WriteString("#\n")
	b.WriteString("#   ignore:\n")
	b.WriteString("#     global:                         # rules applied to every repository\n")
	b.WriteString("#       repos:                        # ignore all events from these repos\n")
	b.WriteString("#         - octocat/noisy-repo\n")
	b.WriteString("#       events:                       # ignore these event types globally\n")
	b.WriteString("#         - PipelineStatusChanged\n")
	b.WriteString("#       except:                       # but never ignore these event:detail pairs\n")
	b.WriteString("#         - PipelineStatusChanged:failed\n")
	b.WriteString("#       authored_by:                  # per-author rules\n")
	b.WriteString("#         - login: renovate[bot]\n")
	b.WriteString("#           events:                   # only suppress these events for this author\n")
	b.WriteString("#             - PipelineStatusChanged\n")
	b.WriteString("#             - ReviewStateChanged\n")
	b.WriteString("#           except:                   # but never suppress these event:detail pairs\n")
	b.WriteString("#             - PipelineStatusChanged:failed\n")
	b.WriteString("#             - ReviewStateChanged:changes_requested\n")
	b.WriteString("#\n")
	b.WriteString("#     octocat/special-repo:           # per-repository overrides\n")
	b.WriteString("#       events:\n")
	b.WriteString("#         - Merged\n")
	b.WriteString("#       authored_by:\n")
	b.WriteString("#         - login: special-bot\n")
	b.WriteString("#           events:\n")
	b.WriteString("#             - Merged\n")
	b.WriteString("#\n")
	b.WriteString("ignore:\n")
	b.WriteString("  global:\n")
	b.WriteString("    repos:\n")
	b.WriteString("    events:\n")
	b.WriteString("    except:\n")
	b.WriteString("    authored_by:\n")
	return b.String()
}

// OpenOrCreateIgnoreFile opens ignore.yaml in the user's editor, creating it if missing.
func OpenOrCreateIgnoreFile(path string) error {
	if err := CreateDefaultIgnoreFile(path); err != nil {
		return err
	}
	return openInEditor(path)
}

// openInEditor opens the given file in the system's default text editor.
func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return execStart(editor, path)
	}
	switch runtime.GOOS {
	case "linux":
		return execStart("xdg-open", path)
	case "darwin":
		return execStart("open", "-t", path)
	case "windows":
		return execStart("notepad", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func execStart(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}
