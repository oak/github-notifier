package pullrequest

import "time"

// Event is the base interface for domain events
type Event interface {
	OccurredAt() time.Time
}

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

// ReviewRequested is raised when a PR review is requested
type ReviewRequested struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	occurredAt    time.Time
}

// NewReviewRequested creates a new event
func NewReviewRequested(pr *PullRequest) ReviewRequested {
	return ReviewRequested{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e ReviewRequested) OccurredAt() time.Time {
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
	Activities    []*Activity
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewActivityDetected creates a new event
func NewActivityDetected(pr *PullRequest) ActivityDetected {
	return ActivityDetected{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		Activities:    pr.Activities(),
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
	occurredAt    time.Time
}

// NewClosed creates a new event
func NewClosed(pr *PullRequest) Closed {
	return Closed{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
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
	occurredAt    time.Time
}

// NewMerged creates a new event
func NewMerged(pr *PullRequest) Merged {
	return Merged{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e Merged) OccurredAt() time.Time {
	return e.occurredAt
}
