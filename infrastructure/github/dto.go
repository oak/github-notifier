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

// TimelineItemDTO represents a timeline item (comment, review, etc.)
type TimelineItemDTO struct {
	Typename  string    `json:"__typename"`
	CreatedAt time.Time `json:"createdAt"`
	Author    *struct {
		Login string `json:"login"`
	} `json:"author,omitempty"`
	Body   string `json:"body,omitempty"`
	State  string `json:"state,omitempty"` // For reviews
	Commit *struct {
		OID           string    `json:"oid"`
		CommittedDate time.Time `json:"committedDate"`
		Author        *struct {
			User *struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"author"`
	} `json:"commit,omitempty"` // For commits
}

// BatchedTimelineResponse represents the response for batched timeline queries using aliases
type BatchedTimelineResponse struct {
	Data   map[string]interface{} `json:"data"`
	Errors []GraphQLError         `json:"errors"`
}
