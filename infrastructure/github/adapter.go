package github

import (
	"fmt"
	"strings"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/rs/zerolog/log"
)

// Adapter implements the pullrequest.PullRequestRepository interface
type Adapter struct {
	client            *Client
	authenticatedUser string // GitHub login of authenticated user
}

// NewAdapter creates a new GitHub adapter
func NewAdapter(token string) *Adapter {
	client := NewClient(token)

	// Fetch authenticated user login
	authenticatedUser, err := client.FetchAuthenticatedUserLogin()
	if err != nil {
		log.Warn().Msgf("Warning: Failed to fetch authenticated user login: %v. Activity filtering will be disabled.", err)
		authenticatedUser = "" // Empty string = no filtering
	} else {
		log.Info().Msgf("Authenticated as: %s", authenticatedUser)
	}

	return &Adapter{
		client:            client,
		authenticatedUser: authenticatedUser,
	}
}

// NewAdapterWithURL creates a new GitHub adapter with a custom base URL (for testing)
func NewAdapterWithURL(baseURL string) *Adapter {
	client := NewClientWithURL("test-token", baseURL)

	// Fetch authenticated user login
	authenticatedUser, err := client.FetchAuthenticatedUserLogin()
	if err != nil {
		log.Warn().Msgf("Warning: Failed to fetch authenticated user login: %v. Activity filtering will be disabled.", err)
		authenticatedUser = "" // Empty string = no filtering
	} else {
		log.Info().Msgf("Authenticated as: %s", authenticatedUser)
	}

	return &Adapter{
		client:            client,
		authenticatedUser: authenticatedUser,
	}
}

// AuthenticatedUser returns the GitHub login of the authenticated user.
// Used by the application layer (e.g. notification handler, use case) to filter
// self-authored activities when deciding what to notify about.
func (a *Adapter) AuthenticatedUser() string {
	return a.authenticatedUser
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
						commits(last: 1) {
							nodes {
								commit {
									statusCheckRollup {
										state
									}
								}
							}
						}
						latestReviews(first: 20) {
							nodes {
								author { login }
								state
								submittedAt
							}
						}
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
						commits(last: 1) {
							nodes {
								commit {
									statusCheckRollup {
										state
									}
								}
							}
						}
						latestReviews(first: 20) {
							nodes {
								author { login }
								state
								submittedAt
							}
						}
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
						commits(last: 1) {
							nodes {
								commit {
									statusCheckRollup {
										state
									}
								}
							}
						}
						latestReviews(first: 20) {
							nodes {
								author { login }
								state
								submittedAt
							}
						}
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

// FetchPRStatus fetches the current status of a specific PR (open, merged, closed).
// Uses a lightweight GraphQL query to check only the PR state.
func (a *Adapter) FetchPRStatus(owner, repo string, number int) (pullrequest.PRStatus, error) {
	query := fmt.Sprintf(`{
		repository(owner: "%s", name: "%s") {
			pullRequest(number: %d) {
				state
			}
		}
	}`, owner, repo, number)

	resp, err := a.client.ExecuteBatchedTimelineQuery(query)
	if err != nil {
		return pullrequest.StatusOpen, fmt.Errorf("failed to fetch PR state: %w", err)
	}

	repoData, ok := resp.Data["repository"].(map[string]interface{})
	if !ok {
		return pullrequest.StatusOpen, fmt.Errorf("unexpected response structure: missing repository")
	}

	prData, ok := repoData["pullRequest"].(map[string]interface{})
	if !ok {
		return pullrequest.StatusOpen, fmt.Errorf("unexpected response structure: missing pullRequest")
	}

	state, ok := prData["state"].(string)
	if !ok {
		return pullrequest.StatusOpen, fmt.Errorf("unexpected response structure: missing state")
	}

	switch state {
	case "MERGED":
		return pullrequest.StatusMerged, nil
	case "CLOSED":
		return pullrequest.StatusClosed, nil
	case "OPEN":
		return pullrequest.StatusOpen, nil
	default:
		return pullrequest.StatusOpen, fmt.Errorf("unknown PR state: %s", state)
	}
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

		prs, err := toDomainList(response.Data.Search.Nodes)
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

// EnrichWithActivities populates PRs with their activities since the given time.
// Uses batched GraphQL queries to minimize API calls.
// Returns all domain events raised by aggregate commands during enrichment.
func (a *Adapter) EnrichWithActivities(prs []*pullrequest.PullRequest, since time.Time) ([]pullrequest.Event, error) {
	if len(prs) == 0 {
		return nil, nil
	}

	totalActivities := 0
	batchSize := 10 // Query 10 PRs per API call to avoid hitting complexity limits
	var allEvents []pullrequest.Event

	// Process PRs in batches
	for i := 0; i < len(prs); i += batchSize {
		end := i + batchSize
		if end > len(prs) {
			end = len(prs)
		}
		batch := prs[i:end]

		log.Info().Msgf("Fetching activities for batch of %d PRs (batch %d/%d)", len(batch), (i/batchSize)+1, (len(prs)+batchSize-1)/batchSize)

		// Fetch activities for this batch; also collects events from head-commit
		// and pipeline-status updates applied to aggregate during fetching.
		activitiesMap, batchEvents, err := a.fetchBatchedTimelines(batch, since)
		if err != nil {
			log.Error().Msgf("Error fetching batch timeline: %v", err)
			continue
		}
		allEvents = append(allEvents, batchEvents...)

		// Add activities to each PR through the aggregate.
		// Collect the ActivityDetected events returned by AddActivities.
		for _, pr := range batch {
			if activities, found := activitiesMap[pr.URL()]; found {
				allEvents = append(allEvents, pr.AddActivities(activities)...)
				totalActivities += len(activities)
			}
		}
	}

	apiCalls := (len(prs) + batchSize - 1) / batchSize
	log.Info().Msgf("Enriched %d PRs with %d total activities using %d API calls (was %d before batching)", len(prs), totalActivities, apiCalls, len(prs))
	return allEvents, nil
}

// fetchBatchedTimelines fetches timeline items for multiple PRs in a single GraphQL query.
// Returns a map of PR URL -> activities, the domain events raised while applying
// head-commit and pipeline-status updates, and any fetch error.
func (a *Adapter) fetchBatchedTimelines(prs []*pullrequest.PullRequest, since time.Time) (map[string][]*pullrequest.Activity, []pullrequest.Event, error) {
	if len(prs) == 0 {
		return make(map[string][]*pullrequest.Activity), nil, nil
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
			log.Warn().Msgf("Invalid repository name: %s", pr.RepositoryName())
			continue
		}

		alias := fmt.Sprintf("pr%d", i)
		prInfos = append(prInfos, prInfo{pr: pr, alias: alias})

		// Add this PR to the query with an alias
		query += fmt.Sprintf(`
			%s: repository(owner: "%s", name: "%s") {
				pullRequest(number: %d) {
					url
					headRefOid
					commits(last: 1) {
						nodes {
							commit {
								statusCheckRollup {
									state
								}
							}
						}
					}
					timelineItems(first: 50, itemTypes: [ISSUE_COMMENT, PULL_REQUEST_REVIEW, PULL_REQUEST_COMMIT]) {
						nodes {
							__typename
							... on IssueComment {
								createdAt
								author { login }
								body
								reactions(first: 100) {
									nodes {
										content
										user { login }
										createdAt
									}
								}
							}
							... on PullRequestReview {
								createdAt
								author { login }
								body
								state
								reactions(first: 100) {
									nodes {
										content
										user { login }
										createdAt
									}
								}
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
		return nil, nil, fmt.Errorf("failed to fetch batched timelines: %w", err)
	}

	// Parse the response and map activities to PR URLs
	result := make(map[string][]*pullrequest.Activity)
	var batchEvents []pullrequest.Event

	for _, info := range prInfos {
		// Extract the timeline items for this PR using the alias
		if repoData, ok := response.Data[info.alias].(map[string]interface{}); ok {
			if prData, ok := repoData["pullRequest"].(map[string]interface{}); ok {
				// Delegate head commit change detection to the domain aggregate
				if headRefOid, ok := prData["headRefOid"].(string); ok {
					batchEvents = append(batchEvents, info.pr.RecordHeadCommitUpdate(headRefOid)...)
				}

				// Parse pipeline status from statusCheckRollup on the latest commit
				if commits, ok := prData["commits"].(map[string]interface{}); ok {
					if nodes, ok := commits["nodes"].([]interface{}); ok && len(nodes) > 0 {
						if lastNode, ok := nodes[len(nodes)-1].(map[string]interface{}); ok {
							if commit, ok := lastNode["commit"].(map[string]interface{}); ok {
								if rollup, ok := commit["statusCheckRollup"].(map[string]interface{}); ok {
									if state, ok := rollup["state"].(string); ok {
										pipelineStatus := pullrequest.PipelineStatusFromRollup(state)
										batchEvents = append(batchEvents, info.pr.UpdatePipelineStatus(pipelineStatus)...)
									}
								}
							}
						}
					}
				}

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
						activities := toActivityList(info.pr, dtos, since)
						result[info.pr.URL()] = activities
					}
				}
			}
		}
	}

	return result, batchEvents, nil
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

	// Parse reactions
	if reactions, ok := data["reactions"].(map[string]interface{}); ok {
		dto.Reactions = &struct {
			Nodes []struct {
				Content   string    `json:"content"`
				CreatedAt time.Time `json:"createdAt"`
				User      *struct {
					Login string `json:"login"`
				} `json:"user"`
			} `json:"nodes"`
		}{}

		if nodes, ok := reactions["nodes"].([]interface{}); ok {
			for _, nodeData := range nodes {
				if nodeMap, ok := nodeData.(map[string]interface{}); ok {
					reaction := struct {
						Content   string    `json:"content"`
						CreatedAt time.Time `json:"createdAt"`
						User      *struct {
							Login string `json:"login"`
						} `json:"user"`
					}{}

					if content, ok := nodeMap["content"].(string); ok {
						reaction.Content = content
					}
					if createdAt, ok := nodeMap["createdAt"].(string); ok {
						if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
							reaction.CreatedAt = t
						}
					}
					if user, ok := nodeMap["user"].(map[string]interface{}); ok {
						reaction.User = &struct {
							Login string `json:"login"`
						}{}
						if login, ok := user["login"].(string); ok {
							reaction.User.Login = login
						}
					}

					dto.Reactions.Nodes = append(dto.Reactions.Nodes, reaction)
				}
			}
		}
	}

	return dto
}
