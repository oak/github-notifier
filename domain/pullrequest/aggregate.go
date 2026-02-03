package pullrequest

import (
	"time"
)

// PullRequest is the aggregate root for pull request domain
type PullRequest struct {
	identifier        PRIdentifier
	title             string
	repository        RepositoryInfo
	author            Author
	status            PRStatus
	createdAt         time.Time
	isDraft           bool
	activities        []*Activity
	lastActivityAt    time.Time
	lastActivityCheck time.Time // When we last checked for activities
	headCommitSHA     string    // Latest commit SHA (head)
	events            []Event   // Domain events pending publication
}

// NewPullRequest creates a new pull request with validation
func NewPullRequest(
	url string,
	number int,
	title string,
	repository RepositoryInfo,
	author Author,
	createdAt time.Time,
	isDraft bool,
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
		identifier:        identifier,
		title:             title,
		repository:        repository,
		author:            author,
		status:            StatusOpen,
		createdAt:         createdAt,
		isDraft:           isDraft,
		activities:        make([]*Activity, 0),
		lastActivityAt:    createdAt,
		lastActivityCheck: time.Time{}, // Zero value - will be checked immediately on first run
		events:            make([]Event, 0),
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

// IsDraft returns whether the PR is a draft
func (pr *PullRequest) IsDraft() bool {
	return pr.isDraft
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
	if pr.status != StatusClosed {
		oldStatus := pr.status
		pr.status = StatusClosed
		event := NewStatusChanged(pr, oldStatus, StatusClosed)
		pr.raiseEvent(&event)
	}
}

// Merge marks the PR as merged
func (pr *PullRequest) Merge() {
	if pr.status != StatusMerged {
		oldStatus := pr.status
		pr.status = StatusMerged
		event := NewStatusChanged(pr, oldStatus, StatusMerged)
		pr.raiseEvent(&event)
	}
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

// AddActivity adds a new activity to the PR (through the aggregate)
// This maintains the aggregate's consistency boundary
func (pr *PullRequest) AddActivity(activity *Activity) {
	if activity == nil {
		return
	}

	pr.activities = append(pr.activities, activity)

	// Update last activity time if this activity is more recent
	if activity.CreatedAt().After(pr.lastActivityAt) {
		pr.lastActivityAt = activity.CreatedAt()
	}
}

// AddActivities adds multiple activities at once
func (pr *PullRequest) AddActivities(activities []*Activity) {
	for _, activity := range activities {
		pr.AddActivity(activity)
	}
}

// Activities returns all activities for this PR
func (pr *PullRequest) Activities() []*Activity {
	// Return a copy to maintain encapsulation
	result := make([]*Activity, len(pr.activities))
	copy(result, pr.activities)
	return result
}

// ActivitiesSince returns activities that occurred after the given time
func (pr *PullRequest) ActivitiesSince(since time.Time) []*Activity {
	var result []*Activity
	for _, activity := range pr.activities {
		if activity.CreatedAt().After(since) {
			result = append(result, activity)
		}
	}
	return result
}

// HasActivitiesSince returns true if there are any activities after the given time
func (pr *PullRequest) HasActivitiesSince(since time.Time) bool {
	return len(pr.ActivitiesSince(since)) > 0
}

// LastActivityAt returns when the last activity occurred
func (pr *PullRequest) LastActivityAt() time.Time {
	return pr.lastActivityAt
}

// ClearActivities clears all activities (useful for testing or rebuilding state)
func (pr *PullRequest) ClearActivities() {
	pr.activities = make([]*Activity, 0)
	pr.lastActivityAt = pr.createdAt
}

// ShouldCheckForActivities determines if we should fetch activities for this PR
// Uses a two-tier approach:
// - Recent PRs (< recentThreshold old): always check
// - Stale PRs (≥ recentThreshold old): check only if enough time has passed since last check
func (pr *PullRequest) ShouldCheckForActivities(recentThreshold, staleCheckInterval time.Duration) bool {
	now := time.Now()
	prAge := now.Sub(pr.createdAt)

	// Recent PRs: always check
	if prAge < recentThreshold {
		return true
	}

	// Stale PRs: check if enough time passed since last check
	timeSinceLastCheck := now.Sub(pr.lastActivityCheck)
	return timeSinceLastCheck >= staleCheckInterval
}

// UpdateLastActivityCheck updates the timestamp when we last checked for activities
func (pr *PullRequest) UpdateLastActivityCheck() {
	pr.lastActivityCheck = time.Now()
}

// LastActivityCheck returns when we last checked for activities
func (pr *PullRequest) LastActivityCheck() time.Time {
	return pr.lastActivityCheck
}

// HeadCommitSHA returns the current head commit SHA
func (pr *PullRequest) HeadCommitSHA() string {
	return pr.headCommitSHA
}

// HeadCommitChanged checks if the head commit SHA has changed
// Returns false if this is the first time seeing this PR (empty SHA)
func (pr *PullRequest) HeadCommitChanged(newHeadSHA string) bool {
	if pr.headCommitSHA == "" {
		// First time seeing this PR - initialize but don't notify
		return false
	}
	return pr.headCommitSHA != newHeadSHA
}

// UpdateHeadCommitSHA updates the stored head commit SHA
func (pr *PullRequest) UpdateHeadCommitSHA(sha string) {
	pr.headCommitSHA = sha
}

// CollectEvents returns all pending domain events and clears the internal event list
func (pr *PullRequest) CollectEvents() []Event {
	events := pr.events
	pr.events = make([]Event, 0)
	return events
}

// raiseEvent adds a domain event to the internal event list
func (pr *PullRequest) raiseEvent(event Event) {
	pr.events = append(pr.events, event)
}

// MarkAsNewlyDetected marks this PR as newly detected and raises the appropriate event
func (pr *PullRequest) MarkAsNewlyDetected() {
	event := NewNewPullRequestDetected(pr)
	pr.raiseEvent(&event)
}

// RecordNewActivity records that new activity has been detected and raises an event
func (pr *PullRequest) RecordNewActivity() {
	if len(pr.activities) > 0 {
		event := NewActivityDetected(pr)
		pr.raiseEvent(&event)
	}
}
