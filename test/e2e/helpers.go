package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// MockGitHubServer simulates GitHub GraphQL API for E2E tests
type MockGitHubServer struct {
	*httptest.Server
	mu            sync.RWMutex
	prs           []MockPR
	comments      map[int][]MockComment
	reviews       map[int][]MockReview
	commits       map[int][]MockCommit
	latestReviews map[int][]MockLatestReview // PR number -> latest reviews (for search response)
	error         *APIError
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

// MockLatestReview represents a review returned in the search query's latestReviews connection
type MockLatestReview struct {
	Author      string
	State       string // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
	SubmittedAt time.Time
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
		prs:           []MockPR{},
		comments:      make(map[int][]MockComment),
		reviews:       make(map[int][]MockReview),
		commits:       make(map[int][]MockCommit),
		latestReviews: make(map[int][]MockLatestReview),
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
	if strings.Contains(request.Query, "viewer") && strings.Contains(request.Query, "login") {
		m.handleViewerQuery(w)
		return
	}

	// Handle batched timeline query (contains aliases like pr0:)
	if strings.Contains(request.Query, "repository(owner:") {
		// Check if it's a PR status query (contains "state" but not "timelineItems")
		if strings.Contains(request.Query, "state") && !strings.Contains(request.Query, "timelineItems") {
			m.handlePRStatusQuery(w, request.Query)
			return
		}
		m.handleBatchedTimelineQuery(w, request.Query)
		return
	}

	// Handle search queries
	if strings.Contains(request.Query, "search") {
		m.handleSearchQuery(w, request.Query)
		return
	}

	// Default empty response
	_ = json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // Test helper
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
	_ = json.NewEncoder(w).Encode(response) //nolint:errcheck // Test helper
}

func (m *MockGitHubServer) handleSearchQuery(w http.ResponseWriter, query string) {
	var filteredPRs []MockPR

	// Filter PRs based on query type
	if strings.Contains(query, "review-requested:@me") || strings.Contains(query, "reviewed-by:@me") {
		// Return PRs where user is requested to review
		for _, pr := range m.prs {
			if pr.State == "open" && pr.Author != "testuser" {
				filteredPRs = append(filteredPRs, pr)
			}
		}
	} else if strings.Contains(query, "author:@me") {
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
		node := map[string]interface{}{
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
		}

		// Add latestReviews if any exist for this PR
		if latestReviews, ok := m.latestReviews[pr.Number]; ok && len(latestReviews) > 0 {
			var reviewNodes []interface{}
			for _, review := range latestReviews {
				reviewNodes = append(reviewNodes, map[string]interface{}{
					"author": map[string]string{
						"login": review.Author,
					},
					"state":       review.State,
					"submittedAt": review.SubmittedAt.Format(time.RFC3339),
				})
			}
			node["latestReviews"] = map[string]interface{}{
				"nodes": reviewNodes,
			}
		}

		nodes = append(nodes, node)
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

	_ = json.NewEncoder(w).Encode(response) //nolint:errcheck // Test helper
}

func (m *MockGitHubServer) handleBatchedTimelineQuery(w http.ResponseWriter, query string) {
	// Parse PR aliases from query (e.g., pr0:, pr1:)
	// For simplicity, we'll return timeline data for all tracked PRs
	// The query format is: pr0: repository(...) { pullRequest(number: N) { ... } }
	// Note: caller already holds m.mu.RLock()

	data := make(map[string]interface{})

	// Parse the query to extract alias → PR number mappings
	// The query format is: pr0: repository(owner: "...", name: "...") { pullRequest(number: N) { ... } }
	// We extract alias and number pairs to match with our mock data
	type aliasMapping struct {
		alias  string
		number int
	}
	var mappings []aliasMapping

	// Use regex to extract "prN: repository..." patterns
	aliasRegex := regexp.MustCompile(`(pr\d+):\s*repository\([^)]*\)\s*\{\s*pullRequest\(number:\s*(\d+)\)`)
	matches := aliasRegex.FindAllStringSubmatch(query, -1)
	for _, match := range matches {
		if len(match) == 3 {
			num := 0
			if _, err := fmt.Sscanf(match[2], "%d", &num); err != nil {
				continue
			}
			mappings = append(mappings, aliasMapping{alias: match[1], number: num})
		}
	}

	// If we couldn't parse the query, fall back to index-based matching
	if len(mappings) == 0 {
		for i, pr := range m.prs {
			mappings = append(mappings, aliasMapping{alias: fmt.Sprintf("pr%d", i), number: pr.Number})
		}
	}

	// Build a lookup from PR number to PR data
	prByNumber := make(map[int]MockPR)
	for _, pr := range m.prs {
		prByNumber[pr.Number] = pr
	}

	// Build response for each alias
	for _, mapping := range mappings {
		pr, ok := prByNumber[mapping.number]
		if !ok {
			continue
		}

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

		data[mapping.alias] = map[string]interface{}{
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

	_ = json.NewEncoder(w).Encode(response) //nolint:errcheck // Test helper
}

func (m *MockGitHubServer) handlePRStatusQuery(w http.ResponseWriter, query string) {
	// Parse PR number from query: pullRequest(number: N)
	prNumRegex := regexp.MustCompile(`pullRequest\(number:\s*(\d+)\)`)
	match := prNumRegex.FindStringSubmatch(query)
	if len(match) < 2 {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // Test helper
			"data": map[string]interface{}{},
		})
		return
	}

	var prNumber int
	if _, err := fmt.Sscanf(match[1], "%d", &prNumber); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // Test helper
			"data": map[string]interface{}{},
		})
		return
	}

	// Find the PR and return its state
	state := "OPEN"
	for _, pr := range m.prs {
		if pr.Number == prNumber {
			switch pr.State {
			case "merged":
				state = "MERGED"
			case "closed":
				state = "CLOSED"
			default:
				state = "OPEN"
			}
			break
		}
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"repository": map[string]interface{}{
				"pullRequest": map[string]interface{}{
					"state": state,
				},
			},
		},
	}

	_ = json.NewEncoder(w).Encode(response) //nolint:errcheck // Test helper
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

// SetLatestReviews sets the latest reviews for a PR (returned in search query's latestReviews connection).
// This is separate from AddReview (which is used for timeline/activity tracking).
// Each call replaces the full set of latest reviews for the PR.
func (m *MockGitHubServer) SetLatestReviews(prNumber int, reviews []MockLatestReview) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range reviews {
		if reviews[i].SubmittedAt.IsZero() {
			reviews[i].SubmittedAt = time.Now()
		}
	}

	m.latestReviews[prNumber] = reviews
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

		// Build a title based on the notification type
		title := "New PR needing review"
		if len(prNotif.StatusChanges) > 0 {
			// Status change notifications take priority
			for _, sc := range prNotif.StatusChanges {
				if sc.EventType == pullrequest.StatusChangeMerged {
					title = "PR Merged"
				} else if sc.EventType == pullrequest.StatusChangeClosed {
					title = "PR Closed"
				}
			}
		} else if !prNotif.IsNew && len(prNotif.ReviewChanges) > 0 {
			title = "PR Review"
		} else if !prNotif.IsNew && len(prNotif.Activities) > 0 {
			title = "PR Activity"
		}

		// Build body with PR info and activities
		body := fmt.Sprintf("%s #%d", pr.Title(), pr.Number())

		// Append review change details to body
		if len(prNotif.ReviewChanges) > 0 {
			var reviewParts []string
			for _, rc := range prNotif.ReviewChanges {
				reviewParts = append(reviewParts, fmt.Sprintf("%s %s", rc.Reviewer, rc.State))
			}
			body += " | " + strings.Join(reviewParts, ", ")
		}

		s.notifications = append(s.notifications, CapturedNotification{
			Title: title,
			Body:  body,
			URL:   pr.URL(),
			Time:  time.Now(),
		})
	}

	return nil
}

// NotifyMessage captures a simple text notification (implements NotificationPort)
func (s *SpyNotificationAdapter) NotifyMessage(title, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.notifications = append(s.notifications, CapturedNotification{
		Title: title,
		Body:  message,
		Time:  time.Now(),
	})

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
