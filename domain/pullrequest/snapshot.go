package pullrequest

import (
	"fmt"
	"time"
)

// PRStateSnapshot is a serializable representation of the mutable state of an
// open pull request. It is used to persist and restore state across process
// restarts so that the application behaves correctly without re-querying
// GitHub for information it already knew.
//
// Activities are intentionally excluded: they are always re-fetched from the
// GitHub API using LastActivityCheck as the fetch window.
type PRStateSnapshot struct {
	URL               string                    `json:"url"`
	Number            int                       `json:"number"`
	Repository        string                    `json:"repository"`
	Author            string                    `json:"author"`
	Title             string                    `json:"title"`
	IsDraft           bool                      `json:"isDraft"`
	CreatedAt         time.Time                 `json:"createdAt"`
	HeadCommitSHA     string                    `json:"headCommitSHA,omitempty"`
	PipelineStatus    PipelineStatus            `json:"pipelineStatus"`
	LastActivityCheck time.Time                 `json:"lastActivityCheck"`
	Reviews           map[string]ReviewSnapshot `json:"reviews"`
}

// ReviewSnapshot is a serializable representation of a single reviewer's
// latest review state on a pull request.
type ReviewSnapshot struct {
	State       ReviewState `json:"state"`
	SubmittedAt time.Time   `json:"submittedAt"`
}

// ToSnapshot exports the current mutable state of the PR into a serializable
// form. The resulting snapshot can be persisted and later passed to
// ReconstitutePRFromSnapshot to rebuild the aggregate without re-fetching from
// GitHub.
func (pr *PullRequest) ToSnapshot() PRStateSnapshot {
	reviews := make(map[string]ReviewSnapshot, len(pr.reviews))
	for login, review := range pr.reviews {
		reviews[login] = ReviewSnapshot{
			State:       review.State(),
			SubmittedAt: review.SubmittedAt(),
		}
	}

	return PRStateSnapshot{
		URL:               pr.identifier.URL(),
		Number:            pr.identifier.Number(),
		Repository:        pr.repository.NameWithOwner(),
		Author:            pr.author.Login(),
		Title:             pr.title,
		IsDraft:           pr.isDraft,
		CreatedAt:         pr.createdAt,
		HeadCommitSHA:     pr.headCommitSHA,
		PipelineStatus:    pr.pipelineStatus,
		LastActivityCheck: pr.lastActivityCheck,
		Reviews:           reviews,
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
func ReconstitutePRFromSnapshot(s PRStateSnapshot) (*PullRequest, error) {
	identifier, err := NewPRIdentifier(s.URL, s.Number)
	if err != nil {
		return nil, fmt.Errorf("reconstitute PR: invalid identifier: %w", err)
	}

	repo, err := NewRepository(s.Repository)
	if err != nil {
		return nil, fmt.Errorf("reconstitute PR: invalid repository: %w", err)
	}

	author, err := NewAuthor(s.Author)
	if err != nil {
		return nil, fmt.Errorf("reconstitute PR: invalid author: %w", err)
	}

	if s.Title == "" {
		return nil, fmt.Errorf("reconstitute PR: title cannot be empty")
	}

	if s.CreatedAt.IsZero() {
		return nil, fmt.Errorf("reconstitute PR: createdAt cannot be zero")
	}

	pr := &PullRequest{
		identifier:        identifier,
		title:             s.Title,
		repository:        repo,
		author:            author,
		status:            StatusOpen,
		createdAt:         s.CreatedAt,
		isDraft:           s.IsDraft,
		activities:        make([]*Activity, 0),
		lastActivityAt:    s.CreatedAt,
		lastActivityCheck: s.LastActivityCheck,
		headCommitSHA:     s.HeadCommitSHA,
		reviews:           make(map[string]*Review),
		pipelineStatus:    s.PipelineStatus,
	}

	for login, rs := range s.Reviews {
		reviewer, err := NewAuthor(login)
		if err != nil {
			return nil, fmt.Errorf("reconstitute PR: invalid reviewer %q: %w", login, err)
		}
		pr.reviews[login] = NewReview(reviewer, rs.State, rs.SubmittedAt)
	}

	return pr, nil
}
