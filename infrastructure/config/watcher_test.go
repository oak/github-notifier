package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchForValidConfig_DetectsValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	// Start with an empty config file
	require.NoError(t, os.WriteFile(path, []byte("# empty config\n"), 0600))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := WatchForValidConfig(ctx, path)

	// Give the watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Write a valid config
	validContent := "GITHUB_TOKEN=ghp_test123\n"
	require.NoError(t, os.WriteFile(path, []byte(validContent), 0600))

	select {
	case cfg := <-ch:
		require.NotNil(t, cfg)
		assert.Equal(t, "ghp_test123", cfg.GitHubToken)
		assert.True(t, cfg.IsValid())
	case <-ctx.Done():
		t.Fatal("Timed out waiting for valid config")
	}
}

func TestWatchForValidConfig_IgnoresInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	// Start with an empty config file
	require.NoError(t, os.WriteFile(path, []byte("# empty config\n"), 0600))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch := WatchForValidConfig(ctx, path)

	// Give the watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Write an invalid config (no token)
	invalidContent := "CHECK_INTERVAL_MINUTES=5\n"
	require.NoError(t, os.WriteFile(path, []byte(invalidContent), 0600))

	// Wait a bit longer than debounce to make sure it was processed
	time.Sleep(1 * time.Second)

	// Now write a valid one
	validContent := "GITHUB_TOKEN=ghp_valid456\nCHECK_INTERVAL_MINUTES=5\n"
	require.NoError(t, os.WriteFile(path, []byte(validContent), 0600))

	select {
	case cfg := <-ch:
		require.NotNil(t, cfg)
		assert.Equal(t, "ghp_valid456", cfg.GitHubToken)
		assert.Equal(t, 5, cfg.CheckInterval)
	case <-ctx.Done():
		t.Fatal("Timed out waiting for valid config")
	}
}

func TestWatchForValidConfig_RespectsContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	require.NoError(t, os.WriteFile(path, []byte("# empty config\n"), 0600))

	ctx, cancel := context.WithCancel(context.Background())
	ch := WatchForValidConfig(ctx, path)

	// Give the watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Cancel before writing valid config
	cancel()

	// Channel should close without sending a config
	cfg, ok := <-ch
	assert.Nil(t, cfg)
	assert.False(t, ok, "channel should be closed after context cancellation")
}

func TestWatchForValidConfig_HandlesNewFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	// Don't create the file initially — LoadConfigWithPath will create a default
	_ = LoadConfigWithPath(path)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := WatchForValidConfig(ctx, path)

	// Give the watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Overwrite with a valid config (simulating user editing the default file)
	validContent := "GITHUB_TOKEN=ghp_newfile789\n"
	require.NoError(t, os.WriteFile(path, []byte(validContent), 0600))

	select {
	case cfg := <-ch:
		require.NotNil(t, cfg)
		assert.Equal(t, "ghp_newfile789", cfg.GitHubToken)
	case <-ctx.Done():
		t.Fatal("Timed out waiting for valid config")
	}
}
