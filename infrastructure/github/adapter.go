package github

import (
	"fmt"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Adapter implements the pullrequest.Repository interface
type Adapter struct {
	client *Client
	mapper *Mapper
}

// NewAdapter creates a new GitHub adapter
func NewAdapter(token string) *Adapter {
	return &Adapter{
		client: NewClient(token),
		mapper: NewMapper(),
	}
}

// FetchRequestedReviews fetches PRs where the user is requested to review
func (a *Adapter) FetchRequestedReviews() ([]*pullrequest.PullRequest, error) {
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

	return a.fetchPaginatedPRs(query)
}

// FetchUserCreated fetches PRs created by the user
func (a *Adapter) FetchUserCreated() ([]*pullrequest.PullRequest, error) {
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

	return a.fetchPaginatedPRs(query)
}

// fetchPaginatedPRs fetches all pages of PRs for a given query
func (a *Adapter) fetchPaginatedPRs(query string) ([]*pullrequest.PullRequest, error) {
	var allPRs []*pullrequest.PullRequest
	var cursor *string

	for {
		variables := map[string]interface{}{
			"cursor": cursor,
		}

		response, err := a.client.ExecuteQuery(query, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch PRs: %w", err)
		}

		prs, err := a.mapper.ToDomainList(response.Data.Search.Nodes)
		if err != nil {
			return nil, fmt.Errorf("failed to map PRs to domain: %w", err)
		}

		allPRs = append(allPRs, prs...)

		if !response.Data.Search.PageInfo.HasNextPage {
			break
		}

		cursor = &response.Data.Search.PageInfo.EndCursor
	}

	return allPRs, nil
}
