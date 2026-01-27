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
}

// NewClient creates a new GitHub API client
func NewClient(token string) *Client {
	transport := &AuthTransport{
		token: token,
		next:  http.DefaultTransport,
	}

	return &Client{
		httpClient: &http.Client{Transport: transport},
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

	req, err := http.NewRequest("POST", githubAPIURL, bytes.NewBuffer(body))
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

	req, err := http.NewRequest("POST", githubAPIURL, bytes.NewBuffer(body))
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
