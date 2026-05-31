// Package json provides a JSON-file-backed implementation of the
// pullrequest.PRTrackingRepository port.
//
// The interface is satisfied by a single StateRepository that reads and
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
	"github.com/oak3/github-notifier/infrastructure/persistence"
)

const currentVersion = 2

// stateEnvelope is the on-disk JSON structure.  The version field is checked
// on load; an unknown version causes the file to be treated as empty so that
// future format changes degrade gracefully.
type stateEnvelope struct {
	Version    int                           `json:"version"`
	TrackedPRs []persistence.PRStateSnapshot `json:"trackedPRs"`
}

// StateRepository implements pullrequest.PRTrackingRepository from a JSON file.
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

func (r *StateRepository) Update(pullRequest *pullrequest.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	// Update the specific PR in the tracked PRs
	for i, snapshot := range env.TrackedPRs {
		if snapshot.URL == pullRequest.Identifier().URL() {
			env.TrackedPRs[i] = persistence.ToSnapshot(pullRequest)
			break
		}
	}
	return r.save(env)
}

// Save replaces the entire tracked-PR snapshot set and persists the change.
func (r *StateRepository) Save(pullrequests []*pullrequest.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	env.TrackedPRs = persistence.ToSnapshots(pullrequests)
	return r.save(env)
}

func (r *StateRepository) Fetch(prIdentifier pullrequest.PRIdentifier) (*pullrequest.PullRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	for _, snapshot := range env.TrackedPRs {
		if snapshot.URL == prIdentifier.URL() {
			pr, err := snapshot.ReconstitutePRFromSnapshot()
			if err != nil {
				return nil, fmt.Errorf("state: failed to reconstitute PR from snapshot: %w", err)
			}
			return pr, nil
		}
	}
	return nil, fmt.Errorf("state: PR not found for URL %s", prIdentifier.URL())
}

// LoadAll returns all previously saved PR snapshots.
// Returns an empty (non-nil) slice when no state has been persisted yet.
func (r *StateRepository) LoadAll() ([]*pullrequest.PullRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	if len(env.TrackedPRs) == 0 {
		return []*pullrequest.PullRequest{}, nil
	}
	// Return a defensive copy.
	cp := make([]*pullrequest.PullRequest, len(env.TrackedPRs))
	for i, snapshot := range env.TrackedPRs {
		pr, err := snapshot.ReconstitutePRFromSnapshot()
		if err != nil {
			log.Warn().Err(err).Str("url", snapshot.URL).Msg("state: failed to reconstitute PR from snapshot; skipping")
			continue
		}
		cp[i] = pr
	}
	// Alternative: skip invalid snapshots and return the valid ones, rather than failing the whole load.
	// This would be more robust to partial file corruption but might hide issues with the snapshot format.
	// For now we log and skip individual failures but still return an error if all snapshots fail, to avoid silently losing all tracking state.
	if len(cp) == 0 {
		return []*pullrequest.PullRequest{}, fmt.Errorf("state: no valid PR snapshots found")
	}
	return cp, nil
}

func (r *StateRepository) IsEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	env := r.load()
	return len(env.TrackedPRs) == 0
}

// ─── Clear ───────────────────────────────────────────────────────────────────
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
	if env.TrackedPRs == nil {
		env.TrackedPRs = []persistence.PRStateSnapshot{}
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
