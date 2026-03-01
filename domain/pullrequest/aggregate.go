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
	lastActivityCheck time.Time          // When we last checked for activities
	headCommitSHA     string             // Latest commit SHA (head)
	reviews           map[string]*Review // reviewer login -> latest review
	pipelineStatus    PipelineStatus     // Latest CI/CD pipeline rollup status
	events            []Event            // Domain events pending publication
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
		reviews:           make(map[string]*Review),
		pipelineStatus:    PipelineStatusUnknown,
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

// Close marks the PR as closed and raises a Closed domain event
func (pr *PullRequest) Close() {
	if pr.status != StatusClosed {
		pr.status = StatusClosed
		event := NewClosed(pr)
		pr.raiseEvent(&event)
	}
}

// Merge marks the PR as merged and raises a Merged domain event
func (pr *PullRequest) Merge() {
	if pr.status != StatusMerged {
		pr.status = StatusMerged
		event := NewMerged(pr)
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

// AddActivity adds a new activity to the PR and raises an ActivityDetected event.
// This maintains the aggregate's consistency boundary and follows the same pattern
// as Close/Merge which raise StatusChanged events.
func (pr *PullRequest) AddActivity(activity *Activity) {
	if activity == nil {
		return
	}

	pr.activities = append(pr.activities, activity)

	// Update last activity time if this activity is more recent
	if activity.CreatedAt().After(pr.lastActivityAt) {
		pr.lastActivityAt = activity.CreatedAt()
	}

	// Raise domain event — activity is a domain fact
	event := NewActivityDetected(pr, activity)
	pr.raiseEvent(&event)
}

// AddActivities adds multiple activities at once
func (pr *PullRequest) AddActivities(activities []*Activity) {
	for _, activity := range activities {
		pr.AddActivity(activity)
	}
}

// SetInitialActivities hydrates the PR with activities during reconstruction (e.g. from the
// GitHub API) without raising domain events. Use this for data loading; use AddActivity /
// AddActivities only when new activity is discovered during business logic.
func (pr *PullRequest) SetInitialActivities(activities []*Activity) {
	for _, activity := range activities {
		if activity == nil {
			continue
		}
		pr.activities = append(pr.activities, activity)
		if activity.CreatedAt().After(pr.lastActivityAt) {
			pr.lastActivityAt = activity.CreatedAt()
		}
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

// SetInitialLastActivityCheck sets the last-activity-check timestamp without
// raising any events. Used to restore known state from a previous cycle when
// hydrating a fresh PR object retrieved from the GitHub API.
func (pr *PullRequest) SetInitialLastActivityCheck(t time.Time) {
	pr.lastActivityCheck = t
}

// LastActivityCheck returns when we last checked for activities
func (pr *PullRequest) LastActivityCheck() time.Time {
	return pr.lastActivityCheck
}

// HeadCommitSHA returns the current head commit SHA
func (pr *PullRequest) HeadCommitSHA() string {
	return pr.headCommitSHA
}

// SetInitialHeadCommitSHA sets the head commit SHA without raising any events.
// Used to restore known state from a previous check cycle.
func (pr *PullRequest) SetInitialHeadCommitSHA(sha string) {
	pr.headCommitSHA = sha
}

// SetInitialPipelineStatus sets the pipeline status without raising any events.
// Used to restore known state from a previous check cycle, so that
// UpdatePipelineStatus can correctly detect changes vs. no-ops.
func (pr *PullRequest) SetInitialPipelineStatus(status PipelineStatus) {
	pr.pipelineStatus = status
}

// RecordHeadCommitUpdate detects if the head commit has changed and, if so,
// creates a push activity on the aggregate (which raises an ActivityDetected event).
// The aggregate records all domain facts; notification filtering (e.g. suppressing
// self-pushes) is handled by the notification event handler.
// First-time initialization (empty current SHA) records the SHA without creating activity.
func (pr *PullRequest) RecordHeadCommitUpdate(newHeadSHA string) {
	if pr.headCommitSHA == "" {
		// First time seeing this PR - initialize but don't create activity
		pr.headCommitSHA = newHeadSHA
		return
	}

	if pr.headCommitSHA == newHeadSHA {
		return // No change
	}

	pr.headCommitSHA = newHeadSHA

	// Create a push activity within the aggregate — this is a domain fact
	pushActivity := NewActivity(
		pr.identifier,
		ActivityTypePush,
		pr.author,
		time.Now(),
		newHeadSHA,
	)
	pr.AddActivity(pushActivity)
}

// DrainEvents returns all pending domain events and clears the internal event list
func (pr *PullRequest) DrainEvents() []Event {
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

// AddReview adds or updates a review for a specific reviewer.
// If the reviewer already has a review with the same state, this is a no-op.
// If the state changed, it raises a ReviewStateChanged event.
func (pr *PullRequest) AddReview(review *Review) {
	if review == nil {
		return
	}

	login := review.Reviewer().Login()
	existing, exists := pr.reviews[login]

	if exists && existing.State() == review.State() {
		// Same state — no change, no event
		return
	}

	// Update or add the review
	pr.reviews[login] = review

	// Raise domain event for state change
	event := NewReviewStateChanged(pr, review.Reviewer(), review.State())
	pr.raiseEvent(&event)
}

// SetInitialReviews sets reviews without raising events.
// Used to restore known state from a previous fetch cycle.
func (pr *PullRequest) SetInitialReviews(reviews map[string]*Review) {
	pr.reviews = reviews
}

// Reviews returns the current reviews map (copy)
func (pr *PullRequest) Reviews() map[string]*Review {
	result := make(map[string]*Review, len(pr.reviews))
	for k, v := range pr.reviews {
		result[k] = v
	}
	return result
}

// PipelineStatus returns the current CI/CD pipeline rollup status
func (pr *PullRequest) PipelineStatus() PipelineStatus {
	return pr.pipelineStatus
}

// UpdatePipelineStatus updates the pipeline status and raises a PipelineStatusChanged event
// if the status has changed. No-op if the status is the same.
func (pr *PullRequest) UpdatePipelineStatus(newStatus PipelineStatus) {
	if pr.pipelineStatus == newStatus {
		return
	}

	oldStatus := pr.pipelineStatus
	pr.pipelineStatus = newStatus

	event := NewPipelineStatusChanged(pr, oldStatus, newStatus)
	pr.raiseEvent(&event)
}

// ReviewSummary returns a ReviewSummary for display purposes
func (pr *PullRequest) ReviewSummary() *ReviewSummary {
	return NewReviewSummary(pr.reviews)
}
