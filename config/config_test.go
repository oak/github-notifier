package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigWithPath_DefaultValues(t *testing.T) {
	// Use a non-existent file so only defaults apply (no env vars set)
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	// Clear all relevant env vars
	clearEnvVars(t)

	cfg := LoadConfigWithPath(path)

	assert.Equal(t, "", cfg.GitHubToken)
	assert.Equal(t, "", cfg.SlackOAuthToken)
	assert.Equal(t, 1, cfg.CheckInterval)
	assert.Equal(t, 100, cfg.MaxNumberOfRepos)
	assert.Equal(t, 100, cfg.MaxNumberOfPRs)
	assert.False(t, cfg.EnableActivityTracking)
	assert.Equal(t, 72, cfg.RecentPRThresholdHours)
	assert.Equal(t, 15, cfg.StalePRCheckIntervalMin)
	assert.True(t, cfg.IncludeDraftPRs)
	assert.Equal(t, "", cfg.MacOSNotificationSender)
	assert.Equal(t, path, cfg.ConfigFilePath)
}

func TestLoadConfigWithPath_CreatesDefaultFileWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	_ = LoadConfigWithPath(path)

	// File should now exist
	_, err := os.Stat(path)
	require.NoError(t, err)

	// Read and verify content
	content, err := os.ReadFile(path)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "# github-notifier configuration file")
	assert.Contains(t, s, "#GITHUB_TOKEN=<your-github-token>")
	assert.Contains(t, s, "#CHECK_INTERVAL_MINUTES=1")
	assert.Contains(t, s, "#ENABLE_ACTIVITY_TRACKING=false")
	assert.Contains(t, s, "#RECENT_PR_THRESHOLD_HOURS=72")
	assert.Contains(t, s, "#STALE_PR_CHECK_INTERVAL=15")
	assert.Contains(t, s, "#INCLUDE_DRAFT_PRS=true")
	assert.Contains(t, s, "#MACOS_NOTIFICATION_SENDER=")
	assert.Contains(t, s, "#SLACK_OAUTH_TOKEN=")
}

func TestLoadConfigWithPath_CreatesFileWithRestrictedPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	_ = LoadConfigWithPath(path)

	info, err := os.Stat(path)
	require.NoError(t, err)

	// 0600 = owner read/write only (secure for tokens)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestLoadConfigWithPath_ReadsFileValues(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	content := `
# My config
GITHUB_TOKEN=file-token-123
SLACK_OAUTH_TOKEN=xoxb-file-token
CHECK_INTERVAL_MINUTES=5
ENABLE_ACTIVITY_TRACKING=true
RECENT_PR_THRESHOLD_HOURS=48
STALE_PR_CHECK_INTERVAL=30
INCLUDE_DRAFT_PRS=false
MACOS_NOTIFICATION_SENDER=com.example.App
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	cfg := LoadConfigWithPath(path)

	assert.Equal(t, "file-token-123", cfg.GitHubToken)
	assert.Equal(t, "xoxb-file-token", cfg.SlackOAuthToken)
	assert.Equal(t, 5, cfg.CheckInterval)
	assert.True(t, cfg.EnableActivityTracking)
	assert.Equal(t, 48, cfg.RecentPRThresholdHours)
	assert.Equal(t, 30, cfg.StalePRCheckIntervalMin)
	assert.False(t, cfg.IncludeDraftPRs)
	assert.Equal(t, "com.example.App", cfg.MacOSNotificationSender)
}

func TestLoadConfigWithPath_EnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	// Write config file with values
	content := `
GITHUB_TOKEN=file-token
CHECK_INTERVAL_MINUTES=5
ENABLE_ACTIVITY_TRACKING=false
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	// Set env vars that should override file values
	t.Setenv("GITHUB_TOKEN", "env-token")
	t.Setenv("CHECK_INTERVAL_MINUTES", "10")
	t.Setenv("ENABLE_ACTIVITY_TRACKING", "true")

	cfg := LoadConfigWithPath(path)

	assert.Equal(t, "env-token", cfg.GitHubToken)
	assert.Equal(t, 10, cfg.CheckInterval)
	assert.True(t, cfg.EnableActivityTracking)
}

func TestLoadConfigWithPath_EnvOnlyWithoutFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	t.Setenv("GITHUB_TOKEN", "env-only-token")
	t.Setenv("STALE_PR_CHECK_INTERVAL", "20")

	cfg := LoadConfigWithPath(path)

	assert.Equal(t, "env-only-token", cfg.GitHubToken)
	assert.Equal(t, 20, cfg.StalePRCheckIntervalMin)
}

func TestLoadConfigWithPath_PartialEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	content := `
GITHUB_TOKEN=file-token
CHECK_INTERVAL_MINUTES=5
RECENT_PR_THRESHOLD_HOURS=48
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	// Only override GITHUB_TOKEN via env
	t.Setenv("GITHUB_TOKEN", "env-token")

	cfg := LoadConfigWithPath(path)

	// Env wins for GITHUB_TOKEN
	assert.Equal(t, "env-token", cfg.GitHubToken)
	// File values for the rest
	assert.Equal(t, 5, cfg.CheckInterval)
	assert.Equal(t, 48, cfg.RecentPRThresholdHours)
	// Default for unset values
	assert.Equal(t, 15, cfg.StalePRCheckIntervalMin)
}

func TestParseConfigLine_KeyValue(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
		wantVal string
		wantOK  bool
	}{
		{"simple", "KEY=value", "KEY", "value", true},
		{"with spaces", " KEY = value ", "KEY", "value", true},
		{"double quoted", `KEY="my value"`, "KEY", "my value", true},
		{"single quoted", `KEY='my value'`, "KEY", "my value", true},
		{"empty value", "KEY=", "KEY", "", true},
		{"no equals", "KEYVALUE", "", "", false},
		{"empty key", "=value", "", "", false},
		{"equals in value", "KEY=val=ue", "KEY", "val=ue", true},
		{"token-like", "GITHUB_TOKEN=ghp_abc123XYZ", "GITHUB_TOKEN", "ghp_abc123XYZ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, val, ok := parseConfigLine(tt.line)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantKey, key)
				assert.Equal(t, tt.wantVal, val)
			}
		})
	}
}

func TestLoadConfigFile_SkipsCommentsAndBlankLines(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	content := `# This is a comment
   # Indented comment

GITHUB_TOKEN=token123

# Another comment
CHECK_INTERVAL_MINUTES=3
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	values := loadConfigFile(path)

	assert.Equal(t, "token123", values["GITHUB_TOKEN"])
	assert.Equal(t, "3", values["CHECK_INTERVAL_MINUTES"])
	assert.Len(t, values, 2)
}

func TestLoadConfigFile_HandlesUnreadableFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	// Create a directory with the same name to make it unreadable as a file
	require.NoError(t, os.MkdirAll(path, 0755))

	values := loadConfigFile(path)
	assert.Empty(t, values)
}

func TestIsValid(t *testing.T) {
	t.Run("valid when token set", func(t *testing.T) {
		cfg := &Config{GitHubToken: "some-token"}
		assert.True(t, cfg.IsValid())
	})

	t.Run("invalid when token empty", func(t *testing.T) {
		cfg := &Config{GitHubToken: ""}
		assert.False(t, cfg.IsValid())
	})
}

func TestGenerateDefaultConfigContent(t *testing.T) {
	content := generateDefaultConfigContent()

	assert.Contains(t, content, "# github-notifier configuration file")
	assert.Contains(t, content, "Environment variables take priority")

	// All config keys should be present as commented-out entries
	for _, entry := range configEntries {
		assert.Contains(t, content, "#"+entry.key+"=")
		assert.Contains(t, content, entry.description)
	}
}

func TestLoadConfigWithPath_DoesNotOverwriteExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	clearEnvVars(t)

	// Write a custom config
	customContent := "GITHUB_TOKEN=my-custom-token\n"
	require.NoError(t, os.WriteFile(path, []byte(customContent), 0600))

	cfg := LoadConfigWithPath(path)
	assert.Equal(t, "my-custom-token", cfg.GitHubToken)

	// File should still have the original content
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, customContent, string(content))
}

func TestResolve_Priority(t *testing.T) {
	t.Run("env wins over file and default", func(t *testing.T) {
		t.Setenv("TEST_KEY", "from-env")
		fileValues := map[string]string{"TEST_KEY": "from-file"}
		assert.Equal(t, "from-env", resolve("TEST_KEY", fileValues, "default"))
	})

	t.Run("file wins over default when no env", func(t *testing.T) {
		clearSingleEnv(t, "TEST_KEY2")
		fileValues := map[string]string{"TEST_KEY2": "from-file"}
		assert.Equal(t, "from-file", resolve("TEST_KEY2", fileValues, "default"))
	})

	t.Run("default used when nothing else", func(t *testing.T) {
		clearSingleEnv(t, "TEST_KEY3")
		fileValues := map[string]string{}
		assert.Equal(t, "default", resolve("TEST_KEY3", fileValues, "default"))
	})
}

func TestResolveInt_Priority(t *testing.T) {
	t.Run("env wins over file and default", func(t *testing.T) {
		t.Setenv("TEST_INT", "42")
		fileValues := map[string]string{"TEST_INT": "10"}
		assert.Equal(t, 42, resolveInt("TEST_INT", fileValues, 1))
	})

	t.Run("file wins over default when no env", func(t *testing.T) {
		clearSingleEnv(t, "TEST_INT2")
		fileValues := map[string]string{"TEST_INT2": "10"}
		assert.Equal(t, 10, resolveInt("TEST_INT2", fileValues, 1))
	})

	t.Run("default used when nothing else", func(t *testing.T) {
		clearSingleEnv(t, "TEST_INT3")
		fileValues := map[string]string{}
		assert.Equal(t, 1, resolveInt("TEST_INT3", fileValues, 1))
	})

	t.Run("falls through on invalid env int", func(t *testing.T) {
		t.Setenv("TEST_INT4", "not-a-number")
		fileValues := map[string]string{"TEST_INT4": "10"}
		assert.Equal(t, 10, resolveInt("TEST_INT4", fileValues, 1))
	})

	t.Run("falls through on invalid file int", func(t *testing.T) {
		clearSingleEnv(t, "TEST_INT5")
		fileValues := map[string]string{"TEST_INT5": "nope"}
		assert.Equal(t, 1, resolveInt("TEST_INT5", fileValues, 1))
	})
}

// clearEnvVars unsets all config-related environment variables for test isolation
func clearEnvVars(t *testing.T) {
	t.Helper()
	envKeys := []string{
		"GITHUB_TOKEN",
		"SLACK_OAUTH_TOKEN",
		"CHECK_INTERVAL_MINUTES",
		"ENABLE_ACTIVITY_TRACKING",
		"RECENT_PR_THRESHOLD_HOURS",
		"STALE_PR_CHECK_INTERVAL",
		"INCLUDE_DRAFT_PRS",
		"MACOS_NOTIFICATION_SENDER",
	}
	for _, key := range envKeys {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

// clearSingleEnv ensures a single env var is not set
func clearSingleEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	os.Unsetenv(key)
}
