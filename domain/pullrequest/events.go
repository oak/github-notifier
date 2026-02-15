package pullrequest

import "time"

// Event is the base interface for domain events
type Event interface {
	OccurredAt() time.Time
}

// EventName constants for event type identification (used by event bus subscriptions)
const (
	EventNewPullRequestDetected = "NewPullRequestDetected"
	EventActivityDetected       = "ActivityDetected"
	EventMerged                 = "Merged"
	EventClosed                 = "Closed"
	EventStatusChanged          = "StatusChanged"
	EventReviewStateChanged     = "ReviewStateChanged"
)

// StatusChangeType represents the type of status change
type StatusChangeType string

const (
	StatusChangeMerged StatusChangeType = "merged"
	StatusChangeClosed StatusChangeType = "closed"
)

// NewPullRequestDetected is raised when a new PR is detected
type NewPullRequestDetected struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	Author        Author
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewNewPullRequestDetected creates a new event
func NewNewPullRequestDetected(pr *PullRequest) NewPullRequestDetected {
	return NewPullRequestDetected{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		Author:        pr.Author(),
		PullRequest:   pr,
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e NewPullRequestDetected) OccurredAt() time.Time {
	return e.occurredAt
}

// StatusChanged is raised when a PR status changes
type StatusChanged struct {
	PullRequestID PRIdentifier
	OldStatus     PRStatus
	NewStatus     PRStatus
	occurredAt    time.Time
}

// NewStatusChanged creates a new event
func NewStatusChanged(pr *PullRequest, oldStatus, newStatus PRStatus) StatusChanged {
	return StatusChanged{
		PullRequestID: pr.Identifier(),
		OldStatus:     oldStatus,
		NewStatus:     newStatus,
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e StatusChanged) OccurredAt() time.Time {
	return e.occurredAt
}

// ActivityDetected is raised when new activity is detected on a PR
type ActivityDetected struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	Activity      *Activity
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewActivityDetected creates a new event for a single activity
func NewActivityDetected(pr *PullRequest, activity *Activity) ActivityDetected {
	return ActivityDetected{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		Activity:      activity,
		PullRequest:   pr,
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e ActivityDetected) OccurredAt() time.Time {
	return e.occurredAt
}

// Closed is raised when a PR is closed
type Closed struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewClosed creates a new event
func NewClosed(pr *PullRequest) Closed {
	return Closed{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		PullRequest:   pr,
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e Closed) OccurredAt() time.Time {
	return e.occurredAt
}

// Merged is raised when a PR is merged
type Merged struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewMerged creates a new event
func NewMerged(pr *PullRequest) Merged {
	return Merged{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		PullRequest:   pr,
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e Merged) OccurredAt() time.Time {
	return e.occurredAt
}

// ReviewStateChanged is raised when a reviewer's review state changes on a PR
type ReviewStateChanged struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	Reviewer      Author
	State         ReviewState
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewReviewStateChanged creates a new event
func NewReviewStateChanged(pr *PullRequest, reviewer Author, state ReviewState) ReviewStateChanged {
	return ReviewStateChanged{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		Reviewer:      reviewer,
		State:         state,
		PullRequest:   pr,
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e ReviewStateChanged) OccurredAt() time.Time {
	return e.occurredAt
}
