package persistence

import (
	"fmt"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// PRStateSnapshot is a serializable representation of the mutable state of an
// open pull request. It is used to persist and restore state across process
// restarts so that the application behaves correctly without re-querying
// GitHub for information it already knew.
//
// Activities are intentionally excluded: they are always re-fetched from the
// GitHub API using LastActivityCheck as the fetch window.
type PRStateSnapshot struct {
	URL               string                     `json:"url"`
	Number            int                        `json:"number"`
	Repository        string                     `json:"repository"`
	Author            string                     `json:"author"`
	Title             string                     `json:"title"`
	IsDraft           bool                       `json:"isDraft"`
	CreatedAt         time.Time                  `json:"createdAt"`
	HeadCommitSHA     string                     `json:"headCommitSHA,omitempty"`
	PipelineStatus    pullrequest.PipelineStatus `json:"pipelineStatus"`
	LastActivityCheck time.Time                  `json:"lastActivityCheck"`
	Reviews           map[string]ReviewSnapshot  `json:"reviews"`
	Seen              bool                       `json:"seen"`
}

// ReviewSnapshot is a serializable representation of a single reviewer's
// latest review state on a pull request.
type ReviewSnapshot struct {
	State       pullrequest.ReviewState `json:"state"`
	SubmittedAt time.Time               `json:"submittedAt"`
}

func ToSnapshots(prs []*pullrequest.PullRequest) []PRStateSnapshot {
	snapshots := make([]PRStateSnapshot, len(prs))
	for i, pr := range prs {
		snapshots[i] = ToSnapshot(pr)
	}
	return snapshots
}

// ToSnapshot exports the current mutable state of the PR into a serializable
// form. The resulting snapshot can be persisted and later passed to
// ReconstitutePRFromSnapshot to rebuild the aggregate without re-fetching from
// GitHub.
func ToSnapshot(pr *pullrequest.PullRequest) PRStateSnapshot {
	reviews := make(map[string]ReviewSnapshot, len(pr.Reviews()))
	for login, review := range pr.Reviews() {
		reviews[login] = ReviewSnapshot{
			State:       review.State(),
			SubmittedAt: review.SubmittedAt(),
		}
	}

	return PRStateSnapshot{
		URL:               pr.Identifier().URL(),
		Number:            pr.Identifier().Number(),
		Repository:        pr.Repository().NameWithOwner(),
		Author:            pr.Author().Login(),
		Title:             pr.Title(),
		IsDraft:           pr.IsDraft(),
		CreatedAt:         pr.CreatedAt(),
		HeadCommitSHA:     pr.HeadCommitSHA(),
		PipelineStatus:    pr.PipelineStatus(),
		LastActivityCheck: pr.LastActivityCheck(),
		Reviews:           reviews,
		Seen:              pr.Seen(),
	}
}

// ReconstitutePRFromSnapshot rebuilds a PullRequest aggregate from a
// previously saved snapshot. Unlike NewPullRequest, this function restores all
// fields directly without raising domain events — it is restoring known state,
// not discovering new facts.
//
// Only open PRs are ever persisted to a snapshot; closed and merged PRs are
// removed from the tracking repository before saving. Status is therefore
// always StatusOpen on reconstitution.
func (s PRStateSnapshot) ReconstitutePRFromSnapshot() (*pullrequest.PullRequest, error) {
	identifier, err := pullrequest.NewPRIdentifier(s.URL, s.Number)
	if err != nil {
		return nil, fmt.Errorf("reconstitute PR: invalid identifier: %w", err)
	}

	repo, err := pullrequest.NewRepository(s.Repository)
	if err != nil {
		return nil, fmt.Errorf("reconstitute PR: invalid repository: %w", err)
	}

	author, err := pullrequest.NewAuthor(s.Author)
	if err != nil {
		return nil, fmt.Errorf("reconstitute PR: invalid author: %w", err)
	}

	if s.Title == "" {
		return nil, fmt.Errorf("reconstitute PR: title cannot be empty")
	}

	if s.CreatedAt.IsZero() {
		return nil, fmt.Errorf("reconstitute PR: createdAt cannot be zero")
	}

	reviews := make(map[string]*pullrequest.Review, len(s.Reviews))
	for login, rs := range s.Reviews {
		reviewer, reviewerErr := pullrequest.NewAuthor(login)
		if reviewerErr != nil {
			return nil, fmt.Errorf("reconstitute PR: invalid reviewer %q: %w", login, reviewerErr)
		}
		reviews[login] = pullrequest.NewReview(reviewer, rs.State, rs.SubmittedAt)
	}

	return pullrequest.ReconstitutePR(
		identifier,
		s.Title,
		repo,
		author,
		pullrequest.StatusOpen,
		s.CreatedAt,
		s.IsDraft,
		make([]*pullrequest.Activity, 0),
		s.LastActivityCheck,
		s.HeadCommitSHA,
		reviews,
		s.PipelineStatus,
		s.Seen,
	), nil
}
