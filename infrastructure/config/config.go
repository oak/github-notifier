package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// DefaultConfigFileName is the name of the config file in the user's home directory
	DefaultConfigFileName = ".github-notifier.conf"

	// DefaultStateFileName is the name of the state file in the user's home directory.
	// It lives alongside the config file and holds the persisted seen/tracked PR state.
	DefaultStateFileName = ".github-notifier.state.json"
)

// Config holds application configuration
type Config struct {
	GitHubToken             string
	SlackOAuthToken         string
	CheckInterval           int // in minutes
	MaxNumberOfRepos        int
	MaxNumberOfPRs          int
	EnableActivityTracking  bool   // Enable checking for comments/reviews/commits (increases API usage)
	RecentPRThresholdHours  int    // PRs created within this are "recent" and checked every minute
	StalePRCheckIntervalMin int    // Check stale PRs every N minutes
	IncludeDraftPRs         bool   // Include draft PRs where user participated (default: true)
	MacOSNotificationSender string // macOS notification sender bundle ID (optional)

	// ConfigFilePath stores the resolved config file path (not a user setting)
	ConfigFilePath string
}

// configEntry defines a config key with its description and default value for file generation
type configEntry struct {
	key          string
	description  string
	defaultValue string
}

// configEntries defines all supported config keys in order, used to generate the default file
var configEntries = []configEntry{
	{
		key:          "GITHUB_TOKEN",
		description:  "GitHub personal access token for API authentication (required)",
		defaultValue: "<your-github-token>",
	},
	{
		key:          "SLACK_OAUTH_TOKEN",
		description:  "Slack OAuth token for sending notifications to Slack DMs (optional)",
		defaultValue: "",
	},
	{
		key:          "CHECK_INTERVAL_MINUTES",
		description:  "How often to check for new PRs in minutes (default: 1, range: 1-60)",
		defaultValue: "1",
	},
	{
		key:          "ENABLE_ACTIVITY_TRACKING",
		description:  "Track comments, reviews, and commits on PRs - increases API usage (default: false)",
		defaultValue: "false",
	},
	{
		key:          "RECENT_PR_THRESHOLD_HOURS",
		description:  "PRs created within this many hours are 'recent' and checked every minute (default: 72)",
		defaultValue: "72",
	},
	{
		key:          "STALE_PR_CHECK_INTERVAL",
		description:  "Check stale PRs every N minutes (default: 15)",
		defaultValue: "15",
	},
	{
		key:          "INCLUDE_DRAFT_PRS",
		description:  "Include draft PRs where you participated (default: true)",
		defaultValue: "true",
	},
	{
		key:          "MACOS_NOTIFICATION_SENDER",
		description:  "macOS notification sender bundle ID (optional, leave empty - sender impersonation is broken on macOS 12+)",
		defaultValue: "",
	},
}

// LoadConfig loads configuration with the following priority:
//  1. Environment variables (highest priority)
//  2. Config file (~/.github-notifier.conf)
//  3. Default values (lowest priority)
//
// If the config file does not exist, it is created with commented-out defaults.
func LoadConfig() *Config {
	return LoadConfigWithPath("")
}

// LoadConfigWithPath loads configuration using a specific config file path.
// If path is empty, it defaults to ~/.github-notifier.conf.
func LoadConfigWithPath(path string) *Config {
	if path == "" {
		path = defaultConfigFilePath()
	}

	fileValues := loadConfigFile(path)

	return &Config{
		GitHubToken:             resolve("GITHUB_TOKEN", fileValues, ""),
		SlackOAuthToken:         resolve("SLACK_OAUTH_TOKEN", fileValues, ""),
		CheckInterval:           resolveInt("CHECK_INTERVAL_MINUTES", fileValues, 1),
		MaxNumberOfRepos:        100,
		MaxNumberOfPRs:          100,
		EnableActivityTracking:  resolve("ENABLE_ACTIVITY_TRACKING", fileValues, "false") == "true",
		RecentPRThresholdHours:  resolveInt("RECENT_PR_THRESHOLD_HOURS", fileValues, 72),
		StalePRCheckIntervalMin: resolveInt("STALE_PR_CHECK_INTERVAL", fileValues, 15),
		IncludeDraftPRs:         resolve("INCLUDE_DRAFT_PRS", fileValues, "true") == "true",
		MacOSNotificationSender: resolve("MACOS_NOTIFICATION_SENDER", fileValues, ""),
		ConfigFilePath:          path,
	}
}

// defaultConfigFilePath returns ~/.github-notifier.conf
func defaultConfigFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfigFileName
	}
	return filepath.Join(home, DefaultConfigFileName)
}

// loadConfigFile reads a config file and returns key-value pairs.
// If the file does not exist, it creates one with commented-out defaults.
// Lines starting with # are comments. Format: KEY=VALUE
func loadConfigFile(path string) map[string]string {
	values := make(map[string]string)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// File doesn't exist — create it with defaults
		if createErr := createDefaultConfigFile(path); createErr != nil {
			// Best effort: if we can't create the file, just use defaults
			return values
		}
		return values
	}

	file, err := os.Open(path)
	if err != nil {
		return values
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := parseConfigLine(line)
		if ok {
			values[key] = value
		}
	}
	if scanner.Err() != nil {
		return make(map[string]string)
	}

	return values
}

// parseConfigLine parses a KEY=VALUE line, handling optional quoting.
func parseConfigLine(line string) (string, string, bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}

	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])

	// Strip surrounding quotes (single or double)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	if key == "" {
		return "", "", false
	}

	return key, value, true
}

// createDefaultConfigFile writes a config file with all settings commented out.
func createDefaultConfigFile(path string) error {
	content := generateDefaultConfigContent()
	return os.WriteFile(path, []byte(content), 0600)
}

// generateDefaultConfigContent returns the content for a new default config file.
func generateDefaultConfigContent() string {
	var b strings.Builder
	b.WriteString("# github-notifier configuration file\n")
	b.WriteString("# Uncomment and set values as needed.\n")
	b.WriteString("# Environment variables take priority over values in this file.\n")
	b.WriteString("\n")

	for i, entry := range configEntries {
		b.WriteString(fmt.Sprintf("# %s\n", entry.description))
		b.WriteString(fmt.Sprintf("#%s=%s\n", entry.key, entry.defaultValue))
		if i < len(configEntries)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// resolve returns the value for a config key with priority: env > file > default
func resolve(key string, fileValues map[string]string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	if value, ok := fileValues[key]; ok {
		return value
	}
	return defaultValue
}

// resolveInt returns the integer value for a config key with priority: env > file > default
func resolveInt(key string, fileValues map[string]string, defaultValue int) int {
	if value, ok := os.LookupEnv(key); ok {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	if value, ok := fileValues[key]; ok {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// IsValid checks if the configuration is valid
func (c *Config) IsValid() bool {
	return c.GitHubToken != ""
}

// StateFilePath returns the path to the JSON state file that persists seen and
// tracked PR state across process restarts.  It is a sibling of ConfigFilePath
// with the name DefaultStateFileName.
func (c *Config) StateFilePath() string {
	return filepath.Join(filepath.Dir(c.ConfigFilePath), DefaultStateFileName)
}
