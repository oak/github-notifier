package pullrequest

import "time"

// Event is the base interface for domain events
type Event interface {
	OccurredAt() time.Time
	Name() string
}

// EventName constants for event type identification (used by event bus subscriptions)
const (
	EventNewPullRequestDetected = "NewPullRequestDetected"
	EventActivityDetected       = "ActivityDetected"
	EventMerged                 = "Merged"
	EventClosed                 = "Closed"
	EventStatusChanged          = "StatusChanged"
	EventReviewStateChanged     = "ReviewStateChanged"
	EventPipelineStatusChanged  = "PipelineStatusChanged"
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

// NewNewPullRequestDetectedAt creates a new event with an explicit timestamp
func NewNewPullRequestDetectedAt(pr *PullRequest, at time.Time) NewPullRequestDetected {
	return NewPullRequestDetected{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		Author:        pr.Author(),
		PullRequest:   pr,
		occurredAt:    at,
	}
}

// NewNewPullRequestDetected creates a new event
func NewNewPullRequestDetected(pr *PullRequest) NewPullRequestDetected {
	return NewNewPullRequestDetectedAt(pr, time.Now())
}

// OccurredAt returns when the event occurred
func (e NewPullRequestDetected) OccurredAt() time.Time {
	return e.occurredAt
}

// Name returns the event name constant
func (e *NewPullRequestDetected) Name() string { return EventNewPullRequestDetected }

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

// Name returns the event name constant
func (e *StatusChanged) Name() string { return EventStatusChanged }

// ActivityDetected is raised when new activity is detected on a PR
type ActivityDetected struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	Activity      *Activity
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewActivityDetectedAt creates a new event with an explicit timestamp
func NewActivityDetectedAt(pr *PullRequest, activity *Activity, at time.Time) ActivityDetected {
	return ActivityDetected{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		Activity:      activity,
		PullRequest:   pr,
		occurredAt:    at,
	}
}

// NewActivityDetected creates a new event for a single activity
func NewActivityDetected(pr *PullRequest, activity *Activity) ActivityDetected {
	return NewActivityDetectedAt(pr, activity, time.Now())
}

// OccurredAt returns when the event occurred
func (e ActivityDetected) OccurredAt() time.Time {
	return e.occurredAt
}

// Name returns the event name constant
func (e *ActivityDetected) Name() string { return EventActivityDetected }

// Closed is raised when a PR is closed
type Closed struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewClosedAt creates a new event with an explicit timestamp
func NewClosedAt(pr *PullRequest, at time.Time) Closed {
	return Closed{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		PullRequest:   pr,
		occurredAt:    at,
	}
}

// NewClosed creates a new event
func NewClosed(pr *PullRequest) Closed {
	return NewClosedAt(pr, time.Now())
}

// OccurredAt returns when the event occurred
func (e Closed) OccurredAt() time.Time {
	return e.occurredAt
}

// Name returns the event name constant
func (e *Closed) Name() string { return EventClosed }

// Merged is raised when a PR is merged
type Merged struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewMergedAt creates a new event with an explicit timestamp
func NewMergedAt(pr *PullRequest, at time.Time) Merged {
	return Merged{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		PullRequest:   pr,
		occurredAt:    at,
	}
}

// NewMerged creates a new event
func NewMerged(pr *PullRequest) Merged {
	return NewMergedAt(pr, time.Now())
}

// OccurredAt returns when the event occurred
func (e Merged) OccurredAt() time.Time {
	return e.occurredAt
}

// Name returns the event name constant
func (e *Merged) Name() string { return EventMerged }

// ReviewStateChanged is raised when a reviewer's review state changes on a PR
type ReviewStateChanged struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	Reviewer      Author
	State         ReviewState
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewReviewStateChangedAt creates a new event with an explicit timestamp
func NewReviewStateChangedAt(pr *PullRequest, reviewer Author, state ReviewState, at time.Time) ReviewStateChanged {
	return ReviewStateChanged{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		Reviewer:      reviewer,
		State:         state,
		PullRequest:   pr,
		occurredAt:    at,
	}
}

// NewReviewStateChanged creates a new event
func NewReviewStateChanged(pr *PullRequest, reviewer Author, state ReviewState) ReviewStateChanged {
	return NewReviewStateChangedAt(pr, reviewer, state, time.Now())
}

// OccurredAt returns when the event occurred
func (e ReviewStateChanged) OccurredAt() time.Time {
	return e.occurredAt
}

// Name returns the event name constant
func (e *ReviewStateChanged) Name() string { return EventReviewStateChanged }

// PipelineStatusChanged is raised when the CI/CD pipeline rollup status changes on a PR
type PipelineStatusChanged struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	OldStatus     PipelineStatus
	NewStatus     PipelineStatus
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewPipelineStatusChangedAt creates a new PipelineStatusChanged event with an explicit timestamp
func NewPipelineStatusChangedAt(pr *PullRequest, oldStatus, newStatus PipelineStatus, at time.Time) PipelineStatusChanged {
	return PipelineStatusChanged{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		OldStatus:     oldStatus,
		NewStatus:     newStatus,
		PullRequest:   pr,
		occurredAt:    at,
	}
}

// NewPipelineStatusChanged creates a new PipelineStatusChanged event
func NewPipelineStatusChanged(pr *PullRequest, oldStatus, newStatus PipelineStatus) PipelineStatusChanged {
	return NewPipelineStatusChangedAt(pr, oldStatus, newStatus, time.Now())
}

// OccurredAt returns when the event occurred
func (e PipelineStatusChanged) OccurredAt() time.Time {
	return e.occurredAt
}

// Name returns the event name constant
func (e *PipelineStatusChanged) Name() string { return EventPipelineStatusChanged }
