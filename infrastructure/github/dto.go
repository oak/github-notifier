package github

import "time"

// PullRequestDTO represents the GitHub API response for a pull request
type PullRequestDTO struct {
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
}

// GraphQLResponse represents the GitHub GraphQL API response
type GraphQLResponse struct {
	Data struct {
		Search struct {
			Nodes []PullRequestDTO `json:"nodes"`
			PageInfo struct {
				EndCursor   string `json:"endCursor"`
				HasNextPage bool   `json:"hasNextPage"`
			} `json:"pageInfo"`
		} `json:"search"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}
