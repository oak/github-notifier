package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// LoadIgnoreConfig loads ignore.yaml from the given path.
// Returns nil without error if the file does not exist.
// Returns nil with an error if the file exists but cannot be parsed.
func LoadIgnoreConfig(path string) (*IgnoreConfig, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg IgnoreConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
