package pullrequest

import (
	"errors"
	"fmt"
	"strings"
)

// PRIdentifier uniquely identifies a pull request
type PRIdentifier struct {
	url    string
	number int
}

// NewPRIdentifier creates a new pull request identifier with validation
func NewPRIdentifier(url string, number int) (PRIdentifier, error) {
	if url == "" {
		return PRIdentifier{}, errors.New("PR URL cannot be empty")
	}
	if number <= 0 {
		return PRIdentifier{}, errors.New("PR number must be positive")
	}
	return PRIdentifier{url: url, number: number}, nil
}

// URL returns the pull request URL
func (id PRIdentifier) URL() string {
	return id.url
}

// Number returns the pull request number
func (id PRIdentifier) Number() int {
	return id.number
}

// Equals compares two PR identifiers
func (id PRIdentifier) Equals(other PRIdentifier) bool {
	return id.url == other.url
}

// String returns a string representation
func (id PRIdentifier) String() string {
	return fmt.Sprintf("PR#%d (%s)", id.number, id.url)
}

// RepositoryInfo represents a GitHub repository
type RepositoryInfo struct {
	nameWithOwner string
}

// NewRepository creates a new repository value object with validation
func NewRepository(nameWithOwner string) (RepositoryInfo, error) {
	if nameWithOwner == "" {
		return RepositoryInfo{}, errors.New("repository name cannot be empty")
	}

	parts := strings.Split(nameWithOwner, "/")
	if len(parts) != 2 {
		return RepositoryInfo{}, fmt.Errorf("invalid repository format: expected 'owner/repo', got '%s'", nameWithOwner)
	}

	if parts[0] == "" || parts[1] == "" {
		return RepositoryInfo{}, errors.New("repository owner and name cannot be empty")
	}

	return RepositoryInfo{nameWithOwner: nameWithOwner}, nil
}

// NameWithOwner returns the full repository name (owner/repo)
func (r RepositoryInfo) NameWithOwner() string {
	return r.nameWithOwner
}

// Owner returns the repository owner
func (r RepositoryInfo) Owner() string {
	parts := strings.Split(r.nameWithOwner, "/")
	return parts[0]
}

// Name returns the repository name
func (r RepositoryInfo) Name() string {
	parts := strings.Split(r.nameWithOwner, "/")
	return parts[1]
}

// Equals compares two repositories
func (r RepositoryInfo) Equals(other RepositoryInfo) bool {
	return r.nameWithOwner == other.nameWithOwner
}

// String returns a string representation
func (r RepositoryInfo) String() string {
	return r.nameWithOwner
}

// Author represents a GitHub user
type Author struct {
	login string
}

// NewAuthor creates a new author value object with validation
func NewAuthor(login string) (Author, error) {
	if login == "" {
		return Author{}, errors.New("author login cannot be empty")
	}

	// GitHub username validation rules
	if len(login) > 39 {
		return Author{}, errors.New("author login cannot exceed 39 characters")
	}

	return Author{login: login}, nil
}

// Login returns the author's GitHub username
func (a Author) Login() string {
	return a.login
}

// Equals compares two authors
func (a Author) Equals(other Author) bool {
	return a.login == other.login
}

// String returns a string representation
func (a Author) String() string {
	return a.login
}

// PRStatus represents the status of a pull request
type PRStatus int

const (
	// StatusOpen indicates the PR is open
	StatusOpen PRStatus = iota
	// StatusMerged indicates the PR has been merged
	StatusMerged
	// StatusClosed indicates the PR has been closed without merging
	StatusClosed
)

// String returns a string representation of the status
func (s PRStatus) String() string {
	switch s {
	case StatusOpen:
		return "open"
	case StatusMerged:
		return "merged"
	case StatusClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// IsOpen returns true if the PR is open
func (s PRStatus) IsOpen() bool {
	return s == StatusOpen
}

// MarshalText implements encoding.TextMarshaler so PRStatus serialises as a
// human-readable string ("open", "merged", "closed") in JSON and other
// text-based formats.
func (s PRStatus) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *PRStatus) UnmarshalText(text []byte) error {
	switch string(text) {
	case "open":
		*s = StatusOpen
	case "merged":
		*s = StatusMerged
	case "closed":
		*s = StatusClosed
	default:
		return fmt.Errorf("unknown PR status %q", string(text))
	}
	return nil
}

// ReviewState represents the state of a pull request review
type ReviewState int

const (
	// ReviewStateApproved indicates the reviewer approved the PR
	ReviewStateApproved ReviewState = iota
	// ReviewStateChangesRequested indicates the reviewer requested changes
	ReviewStateChangesRequested
	// ReviewStateCommented indicates the reviewer left a comment without approving or requesting changes
	ReviewStateCommented
	// ReviewStateDismissed indicates a previous review was dismissed
	ReviewStateDismissed
)

// String returns a string representation of the review state
func (rs ReviewState) String() string {
	switch rs {
	case ReviewStateApproved:
		return "approved"
	case ReviewStateChangesRequested:
		return "changes_requested"
	case ReviewStateCommented:
		return "commented"
	case ReviewStateDismissed:
		return "dismissed"
	default:
		return "unknown"
	}
}

// ReviewStateFromString converts a GitHub API review state string to a ReviewState
func ReviewStateFromString(s string) (ReviewState, bool) {
	switch s {
	case "APPROVED":
		return ReviewStateApproved, true
	case "CHANGES_REQUESTED":
		return ReviewStateChangesRequested, true
	case "COMMENTED":
		return ReviewStateCommented, true
	case "DISMISSED":
		return ReviewStateDismissed, true
	default:
		return 0, false
	}
}

// Emoji returns a display emoji for the review state
func (rs ReviewState) Emoji() string {
	switch rs {
	case ReviewStateApproved:
		return "✅"
	case ReviewStateChangesRequested:
		return "❌"
	case ReviewStateCommented:
		return "💬"
	case ReviewStateDismissed:
		return "🚫"
	default:
		return "?"
	}
}

// Label returns a human-readable label for the review state
func (rs ReviewState) Label() string {
	switch rs {
	case ReviewStateApproved:
		return "approved"
	case ReviewStateChangesRequested:
		return "requested changes"
	case ReviewStateCommented:
		return "commented"
	case ReviewStateDismissed:
		return "dismissed"
	default:
		return "unknown"
	}
}

// MarshalText implements encoding.TextMarshaler so ReviewState serialises as a
// stable lowercase string in JSON and other text-based formats.
func (rs ReviewState) MarshalText() ([]byte, error) {
	return []byte(rs.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (rs *ReviewState) UnmarshalText(text []byte) error {
	switch string(text) {
	case "approved":
		*rs = ReviewStateApproved
	case "changes_requested":
		*rs = ReviewStateChangesRequested
	case "commented":
		*rs = ReviewStateCommented
	case "dismissed":
		*rs = ReviewStateDismissed
	default:
		return fmt.Errorf("unknown review state %q", string(text))
	}
	return nil
}

// PipelineStatus represents the CI/CD pipeline (status check rollup) state of a PR
type PipelineStatus int

const (
	// PipelineStatusUnknown means no pipeline data is available
	PipelineStatusUnknown PipelineStatus = iota
	// PipelineStatusRunning means one or more checks are pending/in progress
	PipelineStatusRunning
	// PipelineStatusSuccess means all checks passed
	PipelineStatusSuccess
	// PipelineStatusFailed means one or more checks failed
	PipelineStatusFailed
)

// String returns a string representation of the pipeline status
func (p PipelineStatus) String() string {
	switch p {
	case PipelineStatusRunning:
		return "running"
	case PipelineStatusSuccess:
		return "success"
	case PipelineStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Emoji returns a display emoji for the pipeline status
func (p PipelineStatus) Emoji() string {
	switch p {
	case PipelineStatusRunning:
		return "🟡"
	case PipelineStatusSuccess:
		return "🟢"
	case PipelineStatusFailed:
		return "🔴"
	case PipelineStatusUnknown:
		return "❓"
	default:
		return "❓"
	}
}

// Label returns a human-readable label for the pipeline status (e.g. "Passed", "Failed")
func (p PipelineStatus) Label() string {
	switch p {
	case PipelineStatusRunning:
		return "Running"
	case PipelineStatusSuccess:
		return "Passed"
	case PipelineStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// MarshalText implements encoding.TextMarshaler so PipelineStatus serialises as
// a stable lowercase string in JSON and other text-based formats.
func (p PipelineStatus) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (p *PipelineStatus) UnmarshalText(text []byte) error {
	switch string(text) {
	case "unknown":
		*p = PipelineStatusUnknown
	case "running":
		*p = PipelineStatusRunning
	case "success":
		*p = PipelineStatusSuccess
	case "failed":
		*p = PipelineStatusFailed
	default:
		return fmt.Errorf("unknown pipeline status %q", string(text))
	}
	return nil
}

// PipelineStatusFromRollup converts a GitHub statusCheckRollup state string to a PipelineStatus
func PipelineStatusFromRollup(state string) PipelineStatus {
	switch state {
	case "PENDING", "IN_PROGRESS", "WAITING", "QUEUED", "ACTION_REQUIRED", "STALE":
		return PipelineStatusRunning
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return PipelineStatusSuccess
	case "FAILURE", "ERROR", "CANCELLED", "TIMED_OUT", "STARTUP_FAILURE":
		return PipelineStatusFailed
	default:
		return PipelineStatusUnknown
	}
}
