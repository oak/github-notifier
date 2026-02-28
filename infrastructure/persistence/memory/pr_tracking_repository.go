package memory

import (
	"sync"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// PRTrackingRepository is an in-memory implementation of the
// pullrequest.PRTrackingRepository port. It stores the last saved set of open
// PR snapshots and is replaced wholesale on every Save call, matching the
// semantics expected by the use cases.
//
// This implementation is intentionally stateless across process restarts — it
// exists so the rest of the codebase can be wired and tested before a
// persistent (JSON / SQLite) adapter is introduced.
type PRTrackingRepository struct {
	mu        sync.RWMutex
	snapshots []pullrequest.PRStateSnapshot
}

// NewPRTrackingRepository creates a new in-memory PR tracking repository.
func NewPRTrackingRepository() pullrequest.PRTrackingRepository {
	return &PRTrackingRepository{
		snapshots: nil,
	}
}

// Save replaces the entire stored snapshot set with the provided slice.
func (r *PRTrackingRepository) Save(snapshots []pullrequest.PRStateSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store a defensive copy so callers cannot mutate our internal slice.
	cp := make([]pullrequest.PRStateSnapshot, len(snapshots))
	copy(cp, snapshots)
	r.snapshots = cp
	return nil
}

// LoadAll returns a copy of the stored snapshots. Returns an empty (non-nil)
// slice when nothing has been saved yet.
func (r *PRTrackingRepository) LoadAll() ([]pullrequest.PRStateSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.snapshots) == 0 {
		return []pullrequest.PRStateSnapshot{}, nil
	}

	cp := make([]pullrequest.PRStateSnapshot, len(r.snapshots))
	copy(cp, r.snapshots)
	return cp, nil
}

// Clear removes all stored snapshots.
func (r *PRTrackingRepository) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.snapshots = nil
	return nil
}
