package github

import (
	"fmt"

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
