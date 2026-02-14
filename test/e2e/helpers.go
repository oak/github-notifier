package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// MockGitHubServer simulates GitHub GraphQL API for E2E tests
type MockGitHubServer struct {
	*httptest.Server
	mu       sync.RWMutex
	prs      []MockPR
	comments map[int][]MockComment
	reviews  map[int][]MockReview
	commits  map[int][]MockCommit
	error    *APIError
}

// MockPR represents a pull request in the mock server
type MockPR struct {
	Title         string
	URL           string
	Number        int
	CreatedAt     time.Time
	IsDraft       bool
	State         string
	Repository    string
	Author        string
	HeadCommitSHA string
}

// MockComment represents a comment
type MockComment struct {
	Author    string
	Body      string
	CreatedAt time.Time
	Reactions []MockReaction
}

// MockReview represents a review
type MockReview struct {
	Author    string
	State     string
	Body      string
	CreatedAt time.Time
	Reactions []MockReaction
}

// MockReaction represents a reaction emoji
type MockReaction struct {
	Content   string // e.g., "THUMBS_UP", "HEART", "LAUGH"
	User      string // Username who reacted
	CreatedAt time.Time
}

// MockCommit represents a commit
type MockCommit struct {
	SHA           string
	Author        string
	CommittedDate time.Time
}

// APIError represents an API error state
type APIError struct {
	Code    int
	Message string
}

// SetupMockGitHubServer creates a new mock GitHub server
func SetupMockGitHubServer() *MockGitHubServer {
	mock := &MockGitHubServer{
		prs:      []MockPR{},
		comments: make(map[int][]MockComment),
		reviews:  make(map[int][]MockReview),
		commits:  make(map[int][]MockCommit),
	}

	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

func (m *MockGitHubServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check for error state
	if m.error != nil {
		http.Error(w, m.error.Message, m.error.Code)
		return
	}

	// Parse GraphQL request
	var request struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Route based on query content
	w.Header().Set("Content-Type", "application/json")

	// Handle viewer login query
	if containsString(request.Query, "viewer") && containsString(request.Query, "login") {
		m.handleViewerQuery(w)
		return
	}

	// Handle batched timeline query (contains aliases like pr0:)
	if containsString(request.Query, "repository(owner:") {
		m.handleBatchedTimelineQuery(w, request.Query)
		return
	}

	// Handle search queries
	if containsString(request.Query, "search") {
		m.handleSearchQuery(w, request.Query)
		return
	}

	// Default empty response
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": map[string]interface{}{},
	})
}

func (m *MockGitHubServer) handleViewerQuery(w http.ResponseWriter) {
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"viewer": map[string]interface{}{
				"login": "testuser",
			},
		},
	}
	json.NewEncoder(w).Encode(response)
}

func (m *MockGitHubServer) handleSearchQuery(w http.ResponseWriter, query string) {
	var filteredPRs []MockPR

	// Filter PRs based on query type
	if containsString(query, "review-requested:@me") || containsString(query, "reviewed-by:@me") {
		// Return PRs where user is requested to review
		for _, pr := range m.prs {
			if pr.State == "open" && pr.Author != "testuser" {
				filteredPRs = append(filteredPRs, pr)
			}
		}
	} else if containsString(query, "author:@me") {
		// Return PRs created by user
		for _, pr := range m.prs {
			if pr.State == "open" && pr.Author == "testuser" {
				filteredPRs = append(filteredPRs, pr)
			}
		}
	}

	// Build response nodes
	var nodes []interface{}
	for _, pr := range filteredPRs {
		nodes = append(nodes, map[string]interface{}{
			"title":     pr.Title,
			"url":       pr.URL,
			"number":    pr.Number,
			"createdAt": pr.CreatedAt.Format(time.RFC3339),
			"isDraft":   pr.IsDraft,
			"repository": map[string]string{
				"nameWithOwner": pr.Repository,
			},
			"author": map[string]string{
				"login": pr.Author,
			},
		})
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{
				"nodes": nodes,
				"pageInfo": map[string]interface{}{
					"endCursor":   "end",
					"hasNextPage": false,
				},
			},
		},
	}

	json.NewEncoder(w).Encode(response)
}

func (m *MockGitHubServer) handleBatchedTimelineQuery(w http.ResponseWriter, query string) {
	// Parse PR aliases from query (e.g., pr0:, pr1:)
	// For simplicity, we'll return timeline data for all tracked PRs
	// The query format is: pr0: repository(...) { pullRequest(number: N) { ... } }

	m.mu.RLock()
	defer m.mu.RUnlock()

	data := make(map[string]interface{})

	// Extract all pr aliases and match them to our PRs
	// Simple approach: return timeline for pr0, pr1, pr2, etc. based on PR array order
	for i, pr := range m.prs {
		alias := fmt.Sprintf("pr%d", i)

		// Build timeline items
		var timelineNodes []interface{}

		// Add comments
		for _, comment := range m.comments[pr.Number] {
			// Build reactions for this comment
			var reactionNodes []interface{}
			for _, reaction := range comment.Reactions {
				reactionNodes = append(reactionNodes, map[string]interface{}{
					"content":   reaction.Content,
					"createdAt": reaction.CreatedAt.Format(time.RFC3339Nano),
					"user": map[string]string{
						"login": reaction.User,
					},
				})
			}

			timelineNodes = append(timelineNodes, map[string]interface{}{
				"__typename": "IssueComment",
				"createdAt":  comment.CreatedAt.Format(time.RFC3339Nano),
				"author": map[string]string{
					"login": comment.Author,
				},
				"body": comment.Body,
				"reactions": map[string]interface{}{
					"nodes": reactionNodes,
				},
			})
		}

		// Add reviews
		for _, review := range m.reviews[pr.Number] {
			// Build reactions for this review
			var reactionNodes []interface{}
			for _, reaction := range review.Reactions {
				reactionNodes = append(reactionNodes, map[string]interface{}{
					"content":   reaction.Content,
					"createdAt": reaction.CreatedAt.Format(time.RFC3339Nano),
					"user": map[string]string{
						"login": reaction.User,
					},
				})
			}

			timelineNodes = append(timelineNodes, map[string]interface{}{
				"__typename": "PullRequestReview",
				"createdAt":  review.CreatedAt.Format(time.RFC3339Nano),
				"author": map[string]string{
					"login": review.Author,
				},
				"body":  review.Body,
				"state": review.State,
				"reactions": map[string]interface{}{
					"nodes": reactionNodes,
				},
			})
		}

		// Add commits
		for _, commit := range m.commits[pr.Number] {
			timelineNodes = append(timelineNodes, map[string]interface{}{
				"__typename": "PullRequestCommit",
				"commit": map[string]interface{}{
					"oid":           commit.SHA,
					"committedDate": commit.CommittedDate.Format(time.RFC3339Nano),
					"author": map[string]interface{}{
						"user": map[string]string{
							"login": commit.Author,
						},
					},
				},
			})
		}

		data[alias] = map[string]interface{}{
			"pullRequest": map[string]interface{}{
				"url":        pr.URL,
				"headRefOid": pr.HeadCommitSHA,
				"timelineItems": map[string]interface{}{
					"nodes": timelineNodes,
				},
			},
		}
	}

	response := map[string]interface{}{
		"data": data,
	}

	json.NewEncoder(w).Encode(response)
}

// SetupPRs configures the mock server with PRs
func (m *MockGitHubServer) SetupPRs(prs []MockPR) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set default values
	for i := range prs {
		if prs[i].State == "" {
			prs[i].State = "open"
		}
		if prs[i].Repository == "" {
			prs[i].Repository = "owner/repo"
		}
		if prs[i].URL == "" {
			prs[i].URL = fmt.Sprintf("https://github.com/owner/repo/pull/%d", prs[i].Number)
		}
		if prs[i].CreatedAt.IsZero() {
			prs[i].CreatedAt = time.Now().Add(-1 * time.Hour)
		}
		if prs[i].HeadCommitSHA == "" {
			prs[i].HeadCommitSHA = "abc123"
		}
	}

	m.prs = prs
}

// AddComment adds a comment to a PR
func (m *MockGitHubServer) AddComment(prNumber int, comment MockComment) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now()
	}

	m.comments[prNumber] = append(m.comments[prNumber], comment)
}

// AddReview adds a review to a PR
func (m *MockGitHubServer) AddReview(prNumber int, review MockReview) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if review.CreatedAt.IsZero() {
		review.CreatedAt = time.Now()
	}

	m.reviews[prNumber] = append(m.reviews[prNumber], review)
}

// AddReactionToComment adds a reaction to an existing comment
func (m *MockGitHubServer) AddReactionToComment(prNumber int, commentIndex int, reaction MockReaction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if reaction.CreatedAt.IsZero() {
		reaction.CreatedAt = time.Now()
	}

	comments := m.comments[prNumber]
	if commentIndex >= 0 && commentIndex < len(comments) {
		// Must update the struct in place
		comment := comments[commentIndex]
		comment.Reactions = append(comment.Reactions, reaction)
		comments[commentIndex] = comment
		m.comments[prNumber] = comments
	}
}

// AddReactionToReview adds a reaction to an existing review
func (m *MockGitHubServer) AddReactionToReview(prNumber int, reviewIndex int, reaction MockReaction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if reaction.CreatedAt.IsZero() {
		reaction.CreatedAt = time.Now()
	}

	reviews := m.reviews[prNumber]
	if reviewIndex >= 0 && reviewIndex < len(reviews) {
		// Must update the struct in place
		review := reviews[reviewIndex]
		review.Reactions = append(review.Reactions, reaction)
		reviews[reviewIndex] = review
		m.reviews[prNumber] = reviews
	}
}

// AddCommit adds a commit to a PR
func (m *MockGitHubServer) AddCommit(prNumber int, commit MockCommit) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if commit.CommittedDate.IsZero() {
		commit.CommittedDate = time.Now()
	}

	m.commits[prNumber] = append(m.commits[prNumber], commit)

	// Update head commit SHA for the PR
	for i, pr := range m.prs {
		if pr.Number == prNumber {
			m.prs[i].HeadCommitSHA = commit.SHA
			break
		}
	}
}

// MergePR marks a PR as merged
func (m *MockGitHubServer) MergePR(prNumber int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, pr := range m.prs {
		if pr.Number == prNumber {
			m.prs[i].State = "merged"
			break
		}
	}
}

// ClosePR marks a PR as closed
func (m *MockGitHubServer) ClosePR(prNumber int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, pr := range m.prs {
		if pr.Number == prNumber {
			m.prs[i].State = "closed"
			break
		}
	}
}

// SetError sets an error state for the server
func (m *MockGitHubServer) SetError(code int, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.error = &APIError{Code: code, Message: message}
}

// ClearError clears the error state
func (m *MockGitHubServer) ClearError() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.error = nil
}

// SpyNotificationAdapter captures notifications for testing
type SpyNotificationAdapter struct {
	mu            sync.Mutex
	notifications []CapturedNotification
}

// CapturedNotification represents a captured notification
type CapturedNotification struct {
	Title string
	Body  string
	URL   string
	Time  time.Time
}

// NewSpyNotificationAdapter creates a new spy adapter
func NewSpyNotificationAdapter() *SpyNotificationAdapter {
	return &SpyNotificationAdapter{
		notifications: []CapturedNotification{},
	}
}

// NotifyPullRequests captures the notification (implements NotificationPort)
func (s *SpyNotificationAdapter) NotifyPullRequests(notifications []*port.PRNotificationData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Capture one notification per PR with all its activities
	for _, prNotif := range notifications {
		pr := prNotif.PullRequest

		// Build a title based on whether it's new or has activity
		title := "New PR needing review"
		if !prNotif.IsNew && len(prNotif.Activities) > 0 {
			title = "PR Activity"
		}

		// Build body with PR info and activities
		body := fmt.Sprintf("%s #%d", pr.Title(), pr.Number())

		s.notifications = append(s.notifications, CapturedNotification{
			Title: title,
			Body:  body,
			URL:   pr.URL(),
			Time:  time.Now(),
		})
	}

	return nil
}

// NotifyNewPullRequests captures the notification (implements NotificationPort)
// DEPRECATED: Use NotifyPullRequests instead
func (s *SpyNotificationAdapter) NotifyNewPullRequests(title string, prs []*pullrequest.PullRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Capture notification for each PR
	for _, pr := range prs {
		s.notifications = append(s.notifications, CapturedNotification{
			Title: title,
			Body:  pr.Title() + " #" + string(rune('0'+pr.Number())),
			URL:   pr.URL(),
			Time:  time.Now(),
		})
	}

	return nil
}

// SupportsClickActions returns true
func (s *SpyNotificationAdapter) SupportsClickActions() bool {
	return true
}

// GetNotifications returns all captured notifications
func (s *SpyNotificationAdapter) GetNotifications() []CapturedNotification {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]CapturedNotification{}, s.notifications...)
}

// Clear clears captured notifications
func (s *SpyNotificationAdapter) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.notifications = []CapturedNotification{}
}

// SpyUIAdapter captures menu updates for testing
type SpyUIAdapter struct {
	mu      sync.Mutex
	prs     []*pullrequest.PullRequest
	updates int
}

// NewSpyUIAdapter creates a new spy UI adapter
func NewSpyUIAdapter() *SpyUIAdapter {
	return &SpyUIAdapter{
		prs: []*pullrequest.PullRequest{},
	}
}

// UpdateDisplay captures the PRs (implements UIPort)
func (s *SpyUIAdapter) UpdateDisplay(requestedPRs, userPRs []*pullrequest.PullRequest, trackingService *pullrequest.TrackingService) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.prs = append(append([]*pullrequest.PullRequest{}, requestedPRs...), userPRs...)
	s.updates++
}

// GetPRs returns captured PRs
func (s *SpyUIAdapter) GetPRs() []*pullrequest.PullRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]*pullrequest.PullRequest{}, s.prs...)
}

// GetUpdateCount returns the number of updates
func (s *SpyUIAdapter) GetUpdateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.updates
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
