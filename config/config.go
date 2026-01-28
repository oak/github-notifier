package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	GitHubToken             string
	SlackOAuthToken         string
	CheckInterval           int  // in minutes
	MaxNumberOfRepos        int
	MaxNumberOfPRs          int
	EnableActivityTracking  bool // Enable checking for comments/reviews/commits (increases API usage)
	RecentPRThresholdHours  int  // PRs created within this are "recent" and checked every minute
	StalePRCheckIntervalMin int  // Check stale PRs every N minutes
	IncludeDraftPRs         bool // Include draft PRs where user participated (default: true)
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		GitHubToken:             getEnv("GITHUB_TOKEN", ""),
		SlackOAuthToken:         getEnv("SLACK_OAUTH_TOKEN", ""),
		CheckInterval:           1,
		MaxNumberOfRepos:        100,
		MaxNumberOfPRs:          100,
		EnableActivityTracking:  getEnv("ENABLE_ACTIVITY_TRACKING", "false") == "true",
		RecentPRThresholdHours:  getEnvInt("RECENT_PR_THRESHOLD_HOURS", 72),
		StalePRCheckIntervalMin: getEnvInt("STALE_PR_CHECK_INTERVAL", 15),
		IncludeDraftPRs:         getEnv("INCLUDE_DRAFT_PRS", "true") == "true",
	}
}

// getEnv retrieves an environment variable with a fallback
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// getEnvInt retrieves an integer environment variable with a fallback
func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

// IsValid checks if the configuration is valid
func (c *Config) IsValid() bool {
	return c.GitHubToken != ""
}
