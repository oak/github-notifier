package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const githubAPIURL = "https://api.github.com/graphql"

// Client handles HTTP communication with GitHub API
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new GitHub API client
func NewClient(token string) *Client {
	transport := &AuthTransport{
		token: token,
		next:  http.DefaultTransport,
	}

	return &Client{
		httpClient: &http.Client{Transport: transport},
		baseURL:    githubAPIURL,
	}
}

// NewClientWithURL creates a new GitHub API client with a custom base URL (for testing)
func NewClientWithURL(token string, baseURL string) *Client {
	transport := &AuthTransport{
		token: token,
		next:  http.DefaultTransport,
	}

	return &Client{
		httpClient: &http.Client{Transport: transport},
		baseURL:    baseURL,
	}
}

// AuthTransport adds authentication headers to requests
type AuthTransport struct {
	token string
	next  http.RoundTripper
}

// RoundTrip implements http.RoundTripper
func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("Content-Type", "application/json")
	return t.next.RoundTrip(req)
}

// ExecuteQuery executes a GraphQL query
func (c *Client) ExecuteQuery(query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	body, err := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var parsed GraphQLResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for GraphQL errors
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %+v", parsed.Errors)
	}

	return &parsed, nil
}

// ExecuteBatchedTimelineQuery executes a batched GraphQL query with aliases
func (c *Client) ExecuteBatchedTimelineQuery(query string) (*BatchedTimelineResponse, error) {
	body, err := json.Marshal(map[string]interface{}{
		"query": query,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var parsed BatchedTimelineResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for GraphQL errors
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %+v", parsed.Errors)
	}

	return &parsed, nil
}

// FetchAuthenticatedUserLogin fetches the login of the authenticated user
func (c *Client) FetchAuthenticatedUserLogin() (string, error) {
	query := `query { viewer { login } }`

	body, err := json.Marshal(map[string]interface{}{
		"query": query,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal query: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []GraphQLError `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for GraphQL errors
	if len(result.Errors) > 0 {
		return "", fmt.Errorf("graphql error: %+v", result.Errors)
	}

	return result.Data.Viewer.Login, nil
}
