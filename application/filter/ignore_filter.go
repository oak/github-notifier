package filter

import "github.com/oak3/github-notifier/config"

// ActivityIgnoreFilter reports whether an event should be suppressed based on the
// provided IgnoreConfig. It returns true if the event should be ignored.
//
// Parameters:
//   - cfg:         the loaded ignore configuration; must not be nil.
//   - repo:        the repository name in "owner/repo" form.
//   - eventName:   the domain event name constant (e.g. EventPipelineStatusChanged).
//   - author:      the GitHub login of the actor that triggered the event.
//   - eventDetail: an optional sub-type string (e.g. pipeline status, review state,
//     activity type). Used for event:detail matching in Except lists.
func ActivityIgnoreFilter(cfg *config.IgnoreConfig, repo, eventName, author, eventDetail string) bool {
	// 1. Global repo blocklist — ignore everything from these repos.
	for _, r := range cfg.Ignore.Global.Repos {
		if r == repo {
			return true
		}
	}

	// 2. Per-repo scope (overrides global for this repo).
	if repoScope, ok := cfg.Ignore.Repos[repo]; ok {
		if matchScope(repoScope, eventName, author, eventDetail) {
			return true
		}
		// Per-repo match is definitive — do not fall through to global.
		return false
	}

	// 3. Global scope.
	return matchScope(cfg.Ignore.Global, eventName, author, eventDetail)
}

// matchScope evaluates a single IgnoreScope against an event and returns true if
// the event should be suppressed.
//
// Evaluation order:
//
//	a. Scope-level Events list  — if non-empty and eventName is listed, candidate for ignore.
//	   Scope-level Except list  — if the event:detail pair is listed, veto the ignore.
//	b. Per-author rules         — if the author matches a rule, evaluate that rule's
//	                               Events/Except lists. A rule with an empty Events list
//	                               matches all event types for that author.
func matchScope(scope config.IgnoreScope, eventName, author, eventDetail string) bool {
	// (a) Scope-level event filter.
	if len(scope.Events) > 0 && contains(scope.Events, eventName) {
		if !containsPair(scope.Except, eventName, eventDetail) {
			return true
		}
	}

	// (b) Per-author rules.
	for _, rule := range scope.AuthoredBy {
		if rule.Login != author {
			continue
		}
		// Author matched — check whether this event type is in scope for the rule.
		if len(rule.Events) > 0 && !contains(rule.Events, eventName) {
			continue // rule targets specific events; this one isn't among them
		}
		// Check except list.
		if containsPair(rule.Except, eventName, eventDetail) {
			continue // vetoed by except
		}
		return true
	}

	return false
}

// contains reports whether s is present in the slice.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// containsPair reports whether "eventName:eventDetail" is present in the except slice.
func containsPair(except []string, eventName, eventDetail string) bool {
	pair := eventName + ":" + eventDetail
	for _, ex := range except {
		if ex == pair {
			return true
		}
	}
	return false
}
