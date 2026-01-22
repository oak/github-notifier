package config

import (
	"os"
)

// Config holds application configuration
type Config struct {
	GitHubToken      string
	CheckInterval    int // in minutes
	MaxNumberOfRepos int
	MaxNumberOfPRs   int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		GitHubToken:      getEnv("GITHUB_TOKEN", ""),
		CheckInterval:    1,
		MaxNumberOfRepos: 100,
		MaxNumberOfPRs:   100,
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
