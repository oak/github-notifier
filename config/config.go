package config

import (
	"os"
)

// Config holds application configuration
type Config struct {
	GitHubToken            string
	SlackOAuthToken        string
	CheckInterval          int // in minutes
	MaxNumberOfRepos       int
	MaxNumberOfPRs         int
	EnableActivityTracking bool // Enable checking for comments/reviews/commits (increases API usage)
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		GitHubToken:            getEnv("GITHUB_TOKEN", ""),
		SlackOAuthToken:        getEnv("SLACK_OAUTH_TOKEN", ""),
		CheckInterval:          1,
		MaxNumberOfRepos:       100,
		MaxNumberOfPRs:         100,
		EnableActivityTracking: getEnv("ENABLE_ACTIVITY_TRACKING", "false") == "true",
	}
}

// getEnv retrieves an environment variable with a fallback
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// IsValid checks if the configuration is valid
func (c *Config) IsValid() bool {
	return c.GitHubToken != ""
}
