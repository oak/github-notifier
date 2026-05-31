package memory

import (
	"fmt"
	"sync"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/persistence"
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
	mu  sync.RWMutex
	prs []persistence.PRStateSnapshot
}

// NewPRTrackingRepository creates a new in-memory PR tracking repository.
func NewPRTrackingRepository() pullrequest.PRTrackingRepository {
	return &PRTrackingRepository{
		prs: nil,
	}
}

// Save replaces the entire stored snapshot set with the provided slice.
func (r *PRTrackingRepository) Save(prs []*pullrequest.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store a defensive copy so callers cannot mutate our internal slice.
	cp := make([]*pullrequest.PullRequest, len(prs))
	copy(cp, prs)
	r.prs = persistence.ToSnapshots(cp)
	return nil
}

func (r *PRTrackingRepository) Update(pullRequest *pullrequest.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, snapshot := range r.prs {
		if snapshot.URL == pullRequest.Identifier().URL() {
			r.prs[i] = persistence.ToSnapshot(pullRequest)
			break
		}
	}
	return nil
}

func (r *PRTrackingRepository) Fetch(prIdentifier pullrequest.PRIdentifier) (*pullrequest.PullRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, snapshot := range r.prs {
		if snapshot.URL == prIdentifier.URL() {
			pr, err := snapshot.ReconstitutePRFromSnapshot()
			if err != nil {
				return nil, err
			}
			return pr, nil
		}
	}
	return nil, fmt.Errorf("PR not found for URL %s", prIdentifier.URL())
}

// LoadAll returns a copy of the stored snapshots. Returns an empty (non-nil)
// slice when nothing has been saved yet.
func (r *PRTrackingRepository) LoadAll() ([]*pullrequest.PullRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.prs) == 0 {
		return []*pullrequest.PullRequest{}, nil
	}

	cp := make([]*pullrequest.PullRequest, len(r.prs))
	for i, snapshot := range r.prs {
		pr, err := snapshot.ReconstitutePRFromSnapshot()
		if err != nil {
			return nil, err
		}
		cp[i] = pr
	}
	return cp, nil
}

func (r *PRTrackingRepository) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.prs) == 0
}

// Clear removes all stored snapshots.
func (r *PRTrackingRepository) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.prs = nil
	return nil
}
