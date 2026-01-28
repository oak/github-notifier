package github

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Adapter implements the pullrequest.PullRequestRepository interface
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

// FetchRequestedReviews fetches PRs where the user is requested to review or has reviewed
// Note: GitHub search doesn't support OR operator, so we fetch both separately and deduplicate
func (a *Adapter) FetchRequestedReviews() ([]*pullrequest.PullRequest, error) {
	// Fetch PRs where user is requested as reviewer
	requestedQuery := `
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
						isDraft
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

	requestedPRs, err := a.fetchPaginatedPRs(requestedQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch review-requested PRs: %w", err)
	}

	// Fetch PRs where user has reviewed
	reviewedQuery := `
		query($cursor: String) {
			search(
				query: "is:open is:pr reviewed-by:@me"
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
						isDraft
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

	reviewedPRs, err := a.fetchPaginatedPRs(reviewedQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reviewed-by PRs: %w", err)
	}

	// Deduplicate by PR URL (a PR could be in both lists if still review-requested after reviewing)
	return a.deduplicatePRs(requestedPRs, reviewedPRs), nil
}

// deduplicatePRs merges two PR lists and removes duplicates based on URL
func (a *Adapter) deduplicatePRs(lists ...[]*pullrequest.PullRequest) []*pullrequest.PullRequest {
	seen := make(map[string]bool)
	var result []*pullrequest.PullRequest

	for _, list := range lists {
		for _, pr := range list {
			if !seen[pr.URL()] {
				seen[pr.URL()] = true
				result = append(result, pr)
			}
		}
	}

	return result
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
						isDraft
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

// EnrichWithActivities populates PRs with their activities since the given time
// Uses batched GraphQL queries to minimize API calls
func (a *Adapter) EnrichWithActivities(prs []*pullrequest.PullRequest, since time.Time) error {
	if len(prs) == 0 {
		return nil
	}

	totalActivities := 0
	batchSize := 10 // Query 10 PRs per API call to avoid hitting complexity limits

	// Process PRs in batches
	for i := 0; i < len(prs); i += batchSize {
		end := i + batchSize
		if end > len(prs) {
			end = len(prs)
		}
		batch := prs[i:end]

		log.Printf("Fetching activities for batch of %d PRs (batch %d/%d)", len(batch), (i/batchSize)+1, (len(prs)+batchSize-1)/batchSize)

		// Fetch activities for this batch
		activitiesMap, err := a.fetchBatchedTimelines(batch, since)
		if err != nil {
			log.Printf("Error fetching batch timeline: %v", err)
			continue
		}

		// Add activities to each PR through the aggregate
		for _, pr := range batch {
			if activities, found := activitiesMap[pr.URL()]; found {
				pr.AddActivities(activities)
				totalActivities += len(activities)
			}
		}
	}

	apiCalls := (len(prs) + batchSize - 1) / batchSize
	log.Printf("Enriched %d PRs with %d total activities using %d API calls (was %d before batching)", len(prs), totalActivities, apiCalls, len(prs))
	return nil
}

// fetchBatchedTimelines fetches timeline items for multiple PRs in a single GraphQL query
// Returns a map of PR URL -> activities
func (a *Adapter) fetchBatchedTimelines(prs []*pullrequest.PullRequest, since time.Time) (map[string][]*pullrequest.Activity, error) {
	if len(prs) == 0 {
		return make(map[string][]*pullrequest.Activity), nil
	}

	// Build the batched query using GraphQL aliases
	query := "query {"

	// Group PRs by repository to reduce redundancy
	type prInfo struct {
		pr    *pullrequest.PullRequest
		alias string
	}
	var prInfos []prInfo

	for i, pr := range prs {
		parts := strings.Split(pr.RepositoryName(), "/")
		if len(parts) != 2 {
			log.Printf("Invalid repository name: %s", pr.RepositoryName())
			continue
		}

		alias := fmt.Sprintf("pr%d", i)
		prInfos = append(prInfos, prInfo{pr: pr, alias: alias})

		// Add this PR to the query with an alias
		query += fmt.Sprintf(`
			%s: repository(owner: "%s", name: "%s") {
				pullRequest(number: %d) {
					url
					timelineItems(first: 50, itemTypes: [ISSUE_COMMENT, PULL_REQUEST_REVIEW, PULL_REQUEST_COMMIT]) {
						nodes {
							__typename
							... on IssueComment {
								createdAt
								author { login }
								body
							}
							... on PullRequestReview {
								createdAt
								author { login }
								body
								state
							}
							... on PullRequestCommit {
								commit {
									oid
									committedDate
									author {
										user { login }
									}
								}
							}
						}
					}
				}
			}
		`, alias, parts[0], parts[1], pr.Number())
	}

	query += "}"

	// Execute the batched query
	response, err := a.client.ExecuteBatchedTimelineQuery(query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch batched timelines: %w", err)
	}

	// Parse the response and map activities to PR URLs
	result := make(map[string][]*pullrequest.Activity)

	for _, info := range prInfos {
		// Extract the timeline items for this PR using the alias
		if repoData, ok := response.Data[info.alias].(map[string]interface{}); ok {
			if prData, ok := repoData["pullRequest"].(map[string]interface{}); ok {
				if timelineData, ok := prData["timelineItems"].(map[string]interface{}); ok {
					if nodes, ok := timelineData["nodes"].([]interface{}); ok {
						// Convert to TimelineItemDTO
						var dtos []TimelineItemDTO
						for _, node := range nodes {
							if nodeMap, ok := node.(map[string]interface{}); ok {
								dto := a.parseTimelineItem(nodeMap)
								if dto != nil {
									dtos = append(dtos, *dto)
								}
							}
						}

						// Map to domain activities
						activities := a.mapper.ToActivityList(info.pr, dtos, since)
						result[info.pr.URL()] = activities
					}
				}
			}
		}
	}

	return result, nil
}

// parseTimelineItem converts a raw map to TimelineItemDTO
func (a *Adapter) parseTimelineItem(data map[string]interface{}) *TimelineItemDTO {
	dto := &TimelineItemDTO{}

	if typename, ok := data["__typename"].(string); ok {
		dto.Typename = typename
	}

	if createdAt, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			dto.CreatedAt = t
		}
	}

	if author, ok := data["author"].(map[string]interface{}); ok {
		dto.Author = &struct {
			Login string `json:"login"`
		}{}
		if login, ok := author["login"].(string); ok {
			dto.Author.Login = login
		}
	}

	if body, ok := data["body"].(string); ok {
		dto.Body = body
	}

	if state, ok := data["state"].(string); ok {
		dto.State = state
	}

	if commit, ok := data["commit"].(map[string]interface{}); ok {
		dto.Commit = &struct {
			OID           string    `json:"oid"`
			CommittedDate time.Time `json:"committedDate"`
			Author        *struct {
				User *struct {
					Login string `json:"login"`
				} `json:"user"`
			} `json:"author"`
		}{}

		if oid, ok := commit["oid"].(string); ok {
			dto.Commit.OID = oid
		}
		if committedDate, ok := commit["committedDate"].(string); ok {
			if t, err := time.Parse(time.RFC3339, committedDate); err == nil {
				dto.Commit.CommittedDate = t
			}
		}
	}

	return dto
}
