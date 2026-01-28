package github

import (
	"fmt"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Mapper converts between DTOs and domain models
type Mapper struct{}

// NewMapper creates a new mapper
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToDomain converts a GitHub DTO to a domain PullRequest
func (m *Mapper) ToDomain(dto PullRequestDTO) (*pullrequest.PullRequest, error) {
	repo, err := pullrequest.NewRepository(dto.Repository.NameWithOwner)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	author, err := pullrequest.NewAuthor(dto.Author.Login)
	if err != nil {
		return nil, fmt.Errorf("failed to create author: %w", err)
	}

	pr, err := pullrequest.NewPullRequest(
		dto.URL,
		dto.Number,
		dto.Title,
		repo,
		author,
		dto.CreatedAt,
		dto.IsDraft,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return pr, nil
}

// ToDomainList converts a list of DTOs to domain PullRequests
func (m *Mapper) ToDomainList(dtos []PullRequestDTO) ([]*pullrequest.PullRequest, error) {
	prs := make([]*pullrequest.PullRequest, 0, len(dtos))

	for _, dto := range dtos {
		pr, err := m.ToDomain(dto)
		if err != nil {
			// Log error but continue with other PRs
			continue
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

// ToActivity converts a single timeline item DTO to a domain activity
func (m *Mapper) ToActivity(pr *pullrequest.PullRequest, dto TimelineItemDTO) *pullrequest.Activity {
	switch dto.Typename {
	case "IssueComment":
		if dto.Author == nil {
			return nil
		}
		author, err := pullrequest.NewAuthor(dto.Author.Login)
		if err != nil {
			return nil
		}
		return pullrequest.NewActivity(
			pr.Identifier(),
			pullrequest.ActivityTypeComment,
			author,
			dto.CreatedAt,
			dto.Body,
		)

	case "PullRequestReview":
		if dto.Author == nil {
			return nil
		}
		// Only notify on reviews with comments or approval/changes requested
		if dto.Body != "" || dto.State == "APPROVED" || dto.State == "CHANGES_REQUESTED" {
			author, err := pullrequest.NewAuthor(dto.Author.Login)
			if err != nil {
				return nil
			}
			return pullrequest.NewActivity(
				pr.Identifier(),
				pullrequest.ActivityTypeReview,
				author,
				dto.CreatedAt,
				dto.Body,
			)
		}

	case "PullRequestCommit":
		if dto.Commit != nil {
			// For commits, use the commit date and author from nested structure
			authorLogin := "unknown"
			if dto.Commit.Author != nil && dto.Commit.Author.User != nil {
				authorLogin = dto.Commit.Author.User.Login
			}

			author, err := pullrequest.NewAuthor(authorLogin)
			if err != nil {
				return nil
			}
			return pullrequest.NewActivity(
				pr.Identifier(),
				pullrequest.ActivityTypeCommit,
				author,
				dto.Commit.CommittedDate,
				dto.Commit.OID[:7], // Use short commit SHA as body
			)
		}
	}

	return nil
}

// ToActivityList converts timeline item DTOs to domain activities, filtering by time and authenticated user
func (m *Mapper) ToActivityList(pr *pullrequest.PullRequest, dtos []TimelineItemDTO, since time.Time, authenticatedUser string) []*pullrequest.Activity {
	var activities []*pullrequest.Activity

	for _, dto := range dtos {
		// Skip items older than since time
		if dto.CreatedAt.Before(since) {
			continue
		}

		activity := m.ToActivity(pr, dto)
		if activity != nil {
			// Filter out activities created by the authenticated user
			if authenticatedUser != "" && activity.Author().Login() == authenticatedUser {
				continue // Skip this activity - it's from @me
			}
			activities = append(activities, activity)
		}
	}

	return activities
}
