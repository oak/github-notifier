package memory

import (
	"sync"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/domain/tracking"
)

// SeenPullRequestRepository is an in-memory implementation of the SeenRepository
type SeenPullRequestRepository struct {
	mu   sync.RWMutex
	seen map[string]bool // key is PR URL
}

// NewSeenPullRequestRepository creates a new in-memory seen repository
func NewSeenPullRequestRepository() tracking.SeenRepository {
	return &SeenPullRequestRepository{
		seen: make(map[string]bool),
	}
}

// MarkAsSeen marks a PR as seen
func (r *SeenPullRequestRepository) MarkAsSeen(id pullrequest.PRIdentifier) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seen[id.URL()] = true
	return nil
}

// HasBeenSeen checks if a PR has been seen
func (r *SeenPullRequestRepository) HasBeenSeen(id pullrequest.PRIdentifier) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.seen[id.URL()]
}

// IsEmpty returns true if no PRs have been marked as seen yet
func (r *SeenPullRequestRepository) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.seen) == 0
}

// Clear removes all seen PR records
func (r *SeenPullRequestRepository) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seen = make(map[string]bool)
	return nil
}
