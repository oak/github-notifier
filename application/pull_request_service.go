package application

import (
	"github.com/oak3/github-notifier/domain"
)

// PullRequestService defines the port for pull request operations
type PullRequestService interface {
	FetchRequestedReviewPRs(token string) ([]domain.PullRequest, error)
	FetchUsersPRs(token string) ([]domain.PullRequest, error)
}
