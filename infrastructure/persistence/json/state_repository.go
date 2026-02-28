// Package json provides a JSON-file-backed implementation of the
// pullrequest.SeenRepository and pullrequest.PRTrackingRepository ports.
//
// Both interfaces are satisfied by a single StateRepository that reads and
// writes one versioned JSON file, allowing atomic persistence of all
// cross-restart state in a single os.Rename call.
package json

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

const currentVersion = 1

// stateEnvelope is the on-disk JSON structure.  The version field is checked
// on load; an unknown version causes the file to be treated as empty so that
// future format changes degrade gracefully.
type stateEnvelope struct {
	Version    int                           `json:"version"`
	SeenPRs    []string                      `json:"seenPRs"`
	TrackedPRs []pullrequest.PRStateSnapshot `json:"trackedPRs"`
}

// StateRepository implements both pullrequest.SeenRepository and
// pullrequest.PRTrackingRepository from a single JSON file.
//
// Reads are done by loading the full file on every call (the file is tiny).
// Writes are atomic: the new content is written to a ".tmp" sibling first,
// then renamed into place with os.Rename, which is atomic on Linux and macOS.
//
// If the file is absent or corrupt the repository treats the state as empty
// and logs a warning — it never returns an error on load.
//
// File permissions: 0600.
type StateRepository struct {
	mu       sync.Mutex
	filePath string
}

// NewStateRepository creates a StateRepository backed by the file at filePath.
// The file need not exist; it will be created on the first write.
func NewStateRepository(filePath string) *StateRepository {
	return &StateRepository{filePath: filePath}
}

// ─── SeenRepository ──────────────────────────────────────────────────────────

// MarkAsSeen marks the given PR as seen and persists the change.
func (r *StateRepository) MarkAsSeen(id pullrequest.PRIdentifier) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	if !contains(env.SeenPRs, id.URL()) {
		env.SeenPRs = append(env.SeenPRs, id.URL())
	}
	return r.save(env)
}

// UnmarkAsSeen removes the given PR from the seen set and persists the change.
func (r *StateRepository) UnmarkAsSeen(id pullrequest.PRIdentifier) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	env.SeenPRs = remove(env.SeenPRs, id.URL())
	return r.save(env)
}

// HasBeenSeen reports whether the given PR has been marked as seen.
func (r *StateRepository) HasBeenSeen(id pullrequest.PRIdentifier) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	return contains(env.SeenPRs, id.URL())
}

// IsEmpty reports whether no PRs have been marked as seen yet.
func (r *StateRepository) IsEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	return len(env.SeenPRs) == 0
}

// ─── PRTrackingRepository ────────────────────────────────────────────────────

// Save replaces the entire tracked-PR snapshot set and persists the change.
func (r *StateRepository) Save(snapshots []pullrequest.PRStateSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	env.TrackedPRs = snapshots
	return r.save(env)
}

// LoadAll returns all previously saved PR snapshots.
// Returns an empty (non-nil) slice when no state has been persisted yet.
func (r *StateRepository) LoadAll() ([]pullrequest.PRStateSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	if len(env.TrackedPRs) == 0 {
		return []pullrequest.PRStateSnapshot{}, nil
	}
	// Return a defensive copy.
	cp := make([]pullrequest.PRStateSnapshot, len(env.TrackedPRs))
	copy(cp, env.TrackedPRs)
	return cp, nil
}

// ─── Clear (both repositories) ───────────────────────────────────────────────

// Clear removes all seen PR records and tracked PR snapshots, and persists the
// empty state.  It satisfies both the SeenRepository.Clear and
// PRTrackingRepository.Clear contracts.
func (r *StateRepository) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.save(stateEnvelope{Version: currentVersion})
}

// ─── internal ────────────────────────────────────────────────────────────────

// load reads and decodes the state file.  On any error (file not found,
// permission error, JSON decode error, unknown version) it returns an empty
// envelope and logs a warning — callers must not return this as an error to
// their own callers.
func (r *StateRepository) load() stateEnvelope {
	data, err := os.ReadFile(r.filePath)
	if os.IsNotExist(err) {
		// First run — normal, no log needed.
		return stateEnvelope{Version: currentVersion}
	}
	if err != nil {
		log.Warn().Err(err).Str("path", r.filePath).Msg("state: failed to read state file; treating as empty")
		return stateEnvelope{Version: currentVersion}
	}

	var env stateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		log.Warn().Err(err).Str("path", r.filePath).Msg("state: corrupt state file; treating as empty")
		return stateEnvelope{Version: currentVersion}
	}

	if env.Version != currentVersion {
		log.Warn().
			Int("fileVersion", env.Version).
			Int("expected", currentVersion).
			Str("path", r.filePath).
			Msg("state: unknown version in state file; treating as empty")
		return stateEnvelope{Version: currentVersion}
	}

	// Ensure slices are non-nil for consistent behaviour.
	if env.SeenPRs == nil {
		env.SeenPRs = []string{}
	}
	if env.TrackedPRs == nil {
		env.TrackedPRs = []pullrequest.PRStateSnapshot{}
	}

	return env
}

// save atomically writes the envelope to the state file.
// It writes to a ".tmp" sibling first and then renames into place.
func (r *StateRepository) save(env stateEnvelope) error {
	env.Version = currentVersion

	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("state: marshal failed: %w", err)
	}

	// Ensure the directory exists (first run on a fresh machine).
	if err := os.MkdirAll(filepath.Dir(r.filePath), 0700); err != nil {
		return fmt.Errorf("state: create directory failed: %w", err)
	}

	tmpPath := r.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("state: write tmp file failed: %w", err)
	}

	if err := os.Rename(tmpPath, r.filePath); err != nil {
		// Best-effort cleanup of the tmp file.
		_ = os.Remove(tmpPath)
		return fmt.Errorf("state: atomic rename failed: %w", err)
	}

	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func remove(slice []string, s string) []string {
	out := slice[:0:0] // reuse backing array if possible
	for _, v := range slice {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}
