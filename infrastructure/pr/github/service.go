package githubpr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/oak3/github-notifier/application"
	"github.com/oak3/github-notifier/domain"
	"github.com/oak3/github-notifier/infrastructure/pr/github/model"
)

const githubURL = "https://api.github.com/graphql"

// GitHubPullRequestService implements application.PullRequestService
type GitHubPullRequestService struct{}

// NewGitHubPullRequestService creates a new GitHub pull request service
func NewGitHubPullRequestService() application.PullRequestService {
	return &GitHubPullRequestService{}
}

func (s *GitHubPullRequestService) FetchUsersPRs(token string) ([]domain.PullRequest, error) {
	var allPRs []domain.PullRequest
	var cursor *string = nil

	for {
		result, err := s.fetchUserPRPage(token, cursor)
		if err != nil {
			return nil, err
		}

		for _, prNode := range result.Data.Search.Nodes {
			pr := domain.PullRequest{
				Title:     prNode.Title,
				URL:       prNode.URL,
				Number:    prNode.Number,
				CreatedAt: prNode.CreatedAt,
			}
			pr.Repository.NameWithOwner = prNode.Repository.NameWithOwner
			pr.Author.Login = prNode.Author.Login
			allPRs = append(allPRs, pr)
		}

		if !result.Data.Search.PageInfo.HasNextPage {
			break
		}

		cursor = &result.Data.Search.PageInfo.EndCursor
	}

	return allPRs, nil
}

func (s *GitHubPullRequestService) FetchPRsRequestedReviews(token string) ([]domain.PullRequest, error) {
	var allPRs []domain.PullRequest
	var cursor *string = nil

	for {
		result, err := s.fetchPRPage(token, cursor)
		if err != nil {
			return nil, err
		}

		for _, prNode := range result.Data.Search.Nodes {
			pr := domain.PullRequest{
				Title:     prNode.Title,
				URL:       prNode.URL,
				Number:    prNode.Number,
				CreatedAt: prNode.CreatedAt,
			}
			pr.Repository.NameWithOwner = prNode.Repository.NameWithOwner
			pr.Author.Login = prNode.Author.Login
			allPRs = append(allPRs, pr)
		}

		if !result.Data.Search.PageInfo.HasNextPage {
			break
		}

		cursor = &result.Data.Search.PageInfo.EndCursor
	}

	return allPRs, nil
}

func (s *GitHubPullRequestService) fetchUserPRPage(token string, cursor *string) (*GHPRRequestedReviewsResponse, error) {

	query := `
		query($cursor: String) {
			search(
				query: "is:open is:pr author:@me"
				type: ISSUE
				first: 50
				after: $cursor
			) {
				nodes {
					... on PullRequest {
						title
						url
						number
						createdAt
						author { login }
						repository { nameWithOwner }
					}
				}
				pageInfo {
					endCursor
					hasNextPage
				}
			}
		}
	`

	variables := map[string]interface{}{
		"cursor": cursor,
	}

	body, _ := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})

	req, _ := http.NewRequest("POST", githubURL, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var parsed GHPRRequestedReviewsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	// --- IMPORTANT: Check GraphQL errors ---
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %+v", parsed.Errors)
	}

	return &parsed, nil
}

func (s *GitHubPullRequestService) fetchPRPage(token string, cursor *string) (*GHPRRequestedReviewsResponse, error) {

	query := `
		query($cursor: String) {
			search(
				query: "is:open is:pr review-requested:@me"
				type: ISSUE
				first: 50
				after: $cursor
			) {
				nodes {
					... on PullRequest {
						title
						url
						number
						createdAt
						author { login }
						repository { nameWithOwner }
					}
				}
				pageInfo {
					endCursor
					hasNextPage
				}
			}
		}
	`

	variables := map[string]interface{}{
		"cursor": cursor,
	}

	body, _ := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})

	req, _ := http.NewRequest("POST", githubURL, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var parsed GHPRRequestedReviewsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	// --- IMPORTANT: Check GraphQL errors ---
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %+v", parsed.Errors)
	}

	return &parsed, nil
}

type GHPRRequestedReviewsResponse struct {
	Data struct {
		Search struct {
			Nodes []struct {
				Title      string    `json:"title"`
				URL        string    `json:"url"`
				Number     int       `json:"number"`
				CreatedAt  time.Time `json:"createdAt"`
				Repository struct {
					NameWithOwner string `json:"nameWithOwner"`
				} `json:"repository"`
				Author struct {
					Login string `json:"login"`
				} `json:"author"`
			} `json:"nodes"`
			PageInfo struct {
				EndCursor   string `json:"endCursor"`
				HasNextPage bool   `json:"hasNextPage"`
			} `json:"pageInfo"`
		} `json:"search"`
	} `json:"data"`

	Errors []model.GHError `json:"errors"`
}
