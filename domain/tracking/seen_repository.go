package tracking

import (
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// SeenRepository is the port for persisting seen pull requests
type SeenRepository interface {
	// MarkAsSeen marks a PR as seen
	MarkAsSeen(id pullrequest.PRIdentifier) error

	// HasBeenSeen checks if a PR has been seen
	HasBeenSeen(id pullrequest.PRIdentifier) bool

	// Clear removes all seen PR records
	Clear() error
}
