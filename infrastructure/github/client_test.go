package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oak/github-notifier/infrastructure/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_FetchAuthenticatedUserLogin_Success(t *testing.T) {
	// Arrange
	expectedLogin := "testuser"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify query
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Contains(t, body["query"], "viewer")
		assert.Contains(t, body["query"], "login")

		// Send response
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"viewer": map[string]interface{}{
					"login": expectedLogin,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := github.NewClientWithURL("test-token", server.URL)

	// Act
	login, err := client.FetchAuthenticatedUserLogin()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedLogin, login)
}

func TestClient_FetchAuthenticatedUserLogin_GraphQLError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errors": []map[string]interface{}{
				{
					"message": "Bad credentials",
					"type":    "FORBIDDEN",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := github.NewClientWithURL("invalid-token", server.URL)

	// Act
	login, err := client.FetchAuthenticatedUserLogin()

	// Assert
	assert.Error(t, err)
	assert.Empty(t, login)
	assert.Contains(t, err.Error(), "graphql error")
	assert.Contains(t, err.Error(), "Bad credentials")
}

func TestClient_FetchAuthenticatedUserLogin_NetworkError(t *testing.T) {
	// Arrange - Use an invalid URL that will fail
	client := github.NewClientWithURL("test-token", "http://invalid-host-that-does-not-exist:9999")

	// Act
	login, err := client.FetchAuthenticatedUserLogin()

	// Assert
	assert.Error(t, err)
	assert.Empty(t, login)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestClient_ExecuteQuery_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)

		// Verify query and variables
		assert.Contains(t, body["query"], "is:open is:pr")
		variables := body["variables"].(map[string]interface{})
		assert.Nil(t, variables["cursor"])

		// Send response with PR data
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"nodes": []map[string]interface{}{
						{
							"title":     "Test PR",
							"url":       "https://github.com/owner/repo/pull/1",
							"number":    float64(1),
							"createdAt": "2024-01-01T10:00:00Z",
							"isDraft":   false,
							"repository": map[string]string{
								"nameWithOwner": "owner/repo",
							},
							"author": map[string]string{
								"login": "testuser",
							},
						},
					},
					"pageInfo": map[string]interface{}{
						"endCursor":   "cursor123",
						"hasNextPage": false,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := github.NewClientWithURL("test-token", server.URL)

	query := `query($cursor: String) {
		search(query: "is:open is:pr", type: ISSUE, first: 50, after: $cursor) {
			nodes { title url }
			pageInfo { endCursor hasNextPage }
		}
	}`
	variables := map[string]interface{}{"cursor": nil}

	// Act
	response, err := client.ExecuteQuery(query, variables)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Data.Search.Nodes, 1)
	assert.Equal(t, "Test PR", response.Data.Search.Nodes[0].Title)
	assert.Equal(t, "https://github.com/owner/repo/pull/1", response.Data.Search.Nodes[0].URL)
	assert.Equal(t, 1, response.Data.Search.Nodes[0].Number)
	assert.Equal(t, "cursor123", response.Data.Search.PageInfo.EndCursor)
	assert.False(t, response.Data.Search.PageInfo.HasNextPage)
}

func TestClient_ExecuteQuery_Pagination(t *testing.T) {
	// Arrange
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		callCount++
		variables := body["variables"].(map[string]interface{})
		cursor := variables["cursor"]

		var response map[string]interface{}
		if cursor == nil {
			// First page
			response = map[string]interface{}{
				"data": map[string]interface{}{
					"search": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{
								"title":      "PR 1",
								"url":        "https://github.com/owner/repo/pull/1",
								"number":     float64(1),
								"createdAt":  "2024-01-01T10:00:00Z",
								"isDraft":    false,
								"repository": map[string]string{"nameWithOwner": "owner/repo"},
								"author":     map[string]string{"login": "user1"},
							},
						},
						"pageInfo": map[string]interface{}{
							"endCursor":   "cursor1",
							"hasNextPage": true,
						},
					},
				},
			}
		} else {
			// Second page
			response = map[string]interface{}{
				"data": map[string]interface{}{
					"search": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{
								"title":      "PR 2",
								"url":        "https://github.com/owner/repo/pull/2",
								"number":     float64(2),
								"createdAt":  "2024-01-02T10:00:00Z",
								"isDraft":    false,
								"repository": map[string]string{"nameWithOwner": "owner/repo"},
								"author":     map[string]string{"login": "user2"},
							},
						},
						"pageInfo": map[string]interface{}{
							"endCursor":   "cursor2",
							"hasNextPage": false,
						},
					},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := github.NewClientWithURL("test-token", server.URL)

	// Act - First call
	query := "query($cursor: String) { search(query: \"test\", after: $cursor) { nodes pageInfo } }"
	response1, err1 := client.ExecuteQuery(query, map[string]interface{}{"cursor": nil})

	// Assert - First page
	require.NoError(t, err1)
	assert.Equal(t, "PR 1", response1.Data.Search.Nodes[0].Title)
	assert.True(t, response1.Data.Search.PageInfo.HasNextPage)
	assert.Equal(t, "cursor1", response1.Data.Search.PageInfo.EndCursor)

	// Act - Second call with cursor
	response2, err2 := client.ExecuteQuery(query, map[string]interface{}{"cursor": "cursor1"})

	// Assert - Second page
	require.NoError(t, err2)
	assert.Equal(t, "PR 2", response2.Data.Search.Nodes[0].Title)
	assert.False(t, response2.Data.Search.PageInfo.HasNextPage)
	assert.Equal(t, 2, callCount)
}

func TestClient_ExecuteQuery_GraphQLError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errors": []map[string]interface{}{
				{
					"message": "Field 'unknown' doesn't exist",
					"type":    "INVALID_QUERY",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := github.NewClientWithURL("test-token", server.URL)

	// Act
	response, err := client.ExecuteQuery("invalid query", nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "graphql error")
}

func TestClient_ExecuteBatchedTimelineQuery_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		query := body["query"].(string)

		// Verify it's a batched query with aliases
		assert.Contains(t, query, "pr0:")
		assert.Contains(t, query, "repository")

		// Send batched response
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"pr0": map[string]interface{}{
					"pullRequest": map[string]interface{}{
						"url":        "https://github.com/owner/repo/pull/1",
						"headRefOid": "abc123",
						"timelineItems": map[string]interface{}{
							"nodes": []map[string]interface{}{
								{
									"__typename": "IssueComment",
									"createdAt":  "2024-01-01T10:00:00Z",
									"author":     map[string]string{"login": "commenter"},
									"body":       "Great work!",
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := github.NewClientWithURL("test-token", server.URL)

	query := `query {
		pr0: repository(owner: "owner", name: "repo") {
			pullRequest(number: 1) {
				url
				headRefOid
				timelineItems(first: 50) { nodes }
			}
		}
	}`

	// Act
	response, err := client.ExecuteBatchedTimelineQuery(query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.Data)

	// Verify the pr0 data exists
	pr0Data, ok := response.Data["pr0"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, pr0Data["pullRequest"])
}

func TestClient_AuthTransport_AddsHeaders(t *testing.T) {
	// Arrange
	expectedToken := "test-token-12345"
	headersCaptured := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers are set correctly
		assert.Equal(t, "Bearer "+expectedToken, r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		headersCaptured = true

		// Send minimal valid response
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"viewer": map[string]interface{}{
					"login": "testuser",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := github.NewClientWithURL(expectedToken, server.URL)

	// Act
	_, err := client.FetchAuthenticatedUserLogin()

	// Assert
	require.NoError(t, err)
	assert.True(t, headersCaptured, "Headers were not verified")
}

func TestClient_InvalidJSON_Response(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := github.NewClientWithURL("test-token", server.URL)

	// Act
	response, err := client.ExecuteQuery("query { test }", nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}
