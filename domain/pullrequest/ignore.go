package pullrequest

// IgnoreConfig holds user-defined rules for suppressing notifications.
// It is a pure domain type with no serialisation concerns; the infrastructure
// layer is responsible for loading it from storage (e.g. ignore.yaml) and
// mapping it to this type before injecting it into the application layer.
type IgnoreConfig struct {
	Ignore IgnoreRuleSet
}

// IgnoreRuleSet contains the global and per-repository ignore rules.
type IgnoreRuleSet struct {
	Global IgnoreScope
	Repos  map[string]IgnoreScope
}

// IgnoreScope holds ignore rules that apply either globally or to a specific repository.
//
//   - Events: ignore all events of these types, regardless of author.
//   - Except: never ignore these specific event:detail combinations even when Events matches.
//   - Repos: (global scope only) ignore all events from these repositories.
//   - AuthoredBy: per-author rules; each entry can further restrict by events/except.
type IgnoreScope struct {
	Events     []string
	Except     []string
	Repos      []string
	AuthoredBy []IgnoreActorRule
}

// IgnoreActorRule filters events from a specific actor (GitHub login).
// Events and Except narrow which event types are suppressed for this actor.
// If Events is empty, all event types from this actor are suppressed (subject to Except).
type IgnoreActorRule struct {
	Login  string
	Events []string
	Except []string
}
