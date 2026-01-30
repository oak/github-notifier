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

// PullRequestReviewRequested is raised when a PR review is requested
type PullRequestReviewRequested struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	occurredAt    time.Time
}

// NewPullRequestReviewRequested creates a new event
func NewPullRequestReviewRequested(pr *PullRequest) PullRequestReviewRequested {
	return PullRequestReviewRequested{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e PullRequestReviewRequested) OccurredAt() time.Time {
	return e.occurredAt
}

// PullRequestStatusChanged is raised when a PR status changes
type PullRequestStatusChanged struct {
	PullRequestID PRIdentifier
	OldStatus     PRStatus
	NewStatus     PRStatus
	occurredAt    time.Time
}

// NewPullRequestStatusChanged creates a new event
func NewPullRequestStatusChanged(pr *PullRequest, oldStatus, newStatus PRStatus) PullRequestStatusChanged {
	return PullRequestStatusChanged{
		PullRequestID: pr.Identifier(),
		OldStatus:     oldStatus,
		NewStatus:     newStatus,
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e PullRequestStatusChanged) OccurredAt() time.Time {
	return e.occurredAt
}

// PullRequestActivityDetected is raised when new activity is detected on a PR
type PullRequestActivityDetected struct {
	PullRequestID PRIdentifier
	Repository    RepositoryInfo
	Activities    []*Activity
	PullRequest   *PullRequest // Full PR for notification purposes
	occurredAt    time.Time
}

// NewPullRequestActivityDetected creates a new event
func NewPullRequestActivityDetected(pr *PullRequest) PullRequestActivityDetected {
	return PullRequestActivityDetected{
		PullRequestID: pr.Identifier(),
		Repository:    pr.Repository(),
		Activities:    pr.Activities(),
		PullRequest:   pr,
		occurredAt:    time.Now(),
	}
}

// OccurredAt returns when the event occurred
func (e PullRequestActivityDetected) OccurredAt() time.Time {
	return e.occurredAt
}
