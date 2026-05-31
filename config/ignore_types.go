package config

// IgnoreConfig represents the structure of the ignore.yaml file
// for filtering out noisy or automated activity.
type IgnoreConfig struct {
	Ignore struct {
		Global IgnoreScope            `yaml:"global"`
		Repos  map[string]IgnoreScope `yaml:",inline"`
	} `yaml:"ignore"`
}

// IgnoreScope holds ignore rules that apply either globally or to a specific repository.
//
// - Events: ignore all events of these types, regardless of author.
// - Except: never ignore these specific event:detail combinations even when Events matches.
// - Repos: (global scope only) ignore all events from these repositories.
// - AuthoredBy: per-author rules; each entry can further restrict by events/except.
type IgnoreScope struct {
	Events     []string          `yaml:"events"`
	Except     []string          `yaml:"except"`
	Repos      []string          `yaml:"repos"`
	AuthoredBy []IgnoreActorRule `yaml:"authored_by"`
}

// IgnoreActorRule filters events from a specific actor (GitHub login).
// Events and Except narrow which event types are suppressed for this actor.
// If Events is empty, all event types from this actor are suppressed (subject to Except).
type IgnoreActorRule struct {
	Login  string   `yaml:"login"`
	Events []string `yaml:"events"`
	Except []string `yaml:"except"`
}
