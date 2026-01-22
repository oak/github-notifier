package pullrequest

import (
	"time"
)

// PullRequest is the aggregate root for pull request domain
type PullRequest struct {
	identifier PRIdentifier
	title      string
	repository RepositoryInfo
	author     Author
	status     PRStatus
	createdAt  time.Time
}

// NewPullRequest creates a new pull request with validation
func NewPullRequest(
	url string,
	number int,
	title string,
	repository RepositoryInfo,
	author Author,
	createdAt time.Time,
) (*PullRequest, error) {
	identifier, err := NewPRIdentifier(url, number)
	if err != nil {
		return nil, err
	}

	if title == "" {
		return nil, ErrInvalidPRIdentifier
	}

	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	return &PullRequest{
		identifier: identifier,
		title:      title,
		repository: repository,
		author:     author,
		status:     StatusOpen,
		createdAt:  createdAt,
	}, nil
}

// Identifier returns the unique identifier for this PR
func (pr *PullRequest) Identifier() PRIdentifier {
	return pr.identifier
}

// Title returns the pull request title
func (pr *PullRequest) Title() string {
	return pr.title
}

// Repository returns the repository this PR belongs to
func (pr *PullRequest) Repository() RepositoryInfo {
	return pr.repository
}

// Author returns the PR author
func (pr *PullRequest) Author() Author {
	return pr.author
}

// Status returns the current status
func (pr *PullRequest) Status() PRStatus {
	return pr.status
}

// CreatedAt returns when the PR was created
func (pr *PullRequest) CreatedAt() time.Time {
	return pr.createdAt
}

// URL returns the PR URL
func (pr *PullRequest) URL() string {
	return pr.identifier.URL()
}

// Number returns the PR number
func (pr *PullRequest) Number() int {
	return pr.identifier.Number()
}

// IsOpen returns true if the PR is open
func (pr *PullRequest) IsOpen() bool {
	return pr.status.IsOpen()
}

// IsStale returns true if the PR is older than the given threshold
func (pr *PullRequest) IsStale(threshold time.Duration) bool {
	return time.Since(pr.createdAt) > threshold
}

// Age returns how long ago the PR was created
func (pr *PullRequest) Age() time.Duration {
	return time.Since(pr.createdAt)
}

// Close marks the PR as closed
func (pr *PullRequest) Close() {
	pr.status = StatusClosed
}

// Merge marks the PR as merged
func (pr *PullRequest) Merge() {
	pr.status = StatusMerged
}

// RepositoryName returns the repository name with owner
func (pr *PullRequest) RepositoryName() string {
	return pr.repository.NameWithOwner()
}

// AuthorLogin returns the author's login
func (pr *PullRequest) AuthorLogin() string {
	return pr.author.Login()
}

// Equals compares two pull requests by their identifier
func (pr *PullRequest) Equals(other *PullRequest) bool {
	return pr.identifier.Equals(other.identifier)
}

// FormattedIdentifier returns a formatted string like "owner/repo#123"
func (pr *PullRequest) FormattedIdentifier() string {
	return pr.repository.NameWithOwner() + "#" + string(rune(pr.identifier.Number()))
}
