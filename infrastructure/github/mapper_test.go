package github

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToDomain_ValidDTO(t *testing.T) {
	// Arrange
	now := time.Now()

	dto := PullRequestDTO{
		Title:     "Test PR",
		URL:       "https://github.com/owner/repo/pull/1",
		Number:    1,
		CreatedAt: now,
		IsDraft:   false,
	}
	dto.Repository.NameWithOwner = "owner/repo"
	dto.Author.Login = "testuser"

	// Act
	pr, err := toDomain(dto)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Test PR", pr.Title())
	assert.Equal(t, 1, pr.Number())
	assert.Equal(t, "https://github.com/owner/repo/pull/1", pr.URL())
	assert.Equal(t, "owner/repo", pr.Repository().NameWithOwner())
	assert.Equal(t, "testuser", pr.Author().Login())
	assert.False(t, pr.IsDraft())
}

func TestToDomain_InvalidRepository(t *testing.T) {
	// Arrange
	dto := PullRequestDTO{
		Title:  "Test PR",
		URL:    "https://github.com/owner/repo/pull/1",
		Number: 1,
	}
	dto.Repository.NameWithOwner = "" // Invalid
	dto.Author.Login = "testuser"

	// Act
	pr, err := toDomain(dto)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "failed to create repository")
}

func TestToDomain_InvalidAuthor(t *testing.T) {
	// Arrange
	dto := PullRequestDTO{
		Title:  "Test PR",
		URL:    "https://github.com/owner/repo/pull/1",
		Number: 1,
	}
	dto.Repository.NameWithOwner = "owner/repo"
	dto.Author.Login = "" // Invalid

	// Act
	pr, err := toDomain(dto)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "failed to create author")
}

func TestToDomainList_ValidDTOs(t *testing.T) {
	// Arrange
	dto1 := createValidDTO(1, "PR 1")
	dto2 := createValidDTO(2, "PR 2")

	// Act
	prs, err := toDomainList([]PullRequestDTO{dto1, dto2})

	// Assert
	require.NoError(t, err)
	assert.Len(t, prs, 2)
	assert.Equal(t, 1, prs[0].Number())
	assert.Equal(t, 2, prs[1].Number())
}

func TestToDomainList_SkipsInvalidDTOs(t *testing.T) {
	// Arrange
	validDTO := createValidDTO(1, "Valid PR")
	invalidDTO := PullRequestDTO{} // Missing required fields

	// Act
	prs, err := toDomainList([]PullRequestDTO{validDTO, invalidDTO})

	// Assert
	require.NoError(t, err)
	assert.Len(t, prs, 1) // Only valid PR
	assert.Equal(t, 1, prs[0].Number())
}

func TestToActivity_Comment(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()

	dto := TimelineItemDTO{
		Typename:  "IssueComment",
		CreatedAt: now,
		Body:      "Test comment",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "commenter"}

	// Act
	activity := toActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeComment, activity.Type())
	assert.Equal(t, "commenter", activity.Author().Login())
	assert.Equal(t, "Test comment", activity.Body())
}

func TestToActivity_Review_WithBody(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	dto := TimelineItemDTO{
		Typename:  "PullRequestReview",
		CreatedAt: time.Now(),
		Body:      "Looks good!",
		State:     "COMMENTED",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "reviewer"}

	// Act
	activity := toActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeReview, activity.Type())
	assert.Equal(t, "reviewer", activity.Author().Login())
}

func TestToActivity_Review_Approved(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	dto := TimelineItemDTO{
		Typename:  "PullRequestReview",
		CreatedAt: time.Now(),
		Body:      "",
		State:     "APPROVED",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "reviewer"}

	// Act
	activity := toActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeReview, activity.Type())
}

func TestToActivity_Review_ChangesRequested(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	dto := TimelineItemDTO{
		Typename:  "PullRequestReview",
		CreatedAt: time.Now(),
		Body:      "",
		State:     "CHANGES_REQUESTED",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "reviewer"}

	// Act
	activity := toActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeReview, activity.Type())
}

func TestToActivity_Review_EmptyBodyNoState_ReturnsNil(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	dto := TimelineItemDTO{
		Typename:  "PullRequestReview",
		CreatedAt: time.Now(),
		Body:      "",
		State:     "COMMENTED",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "reviewer"}

	// Act
	activity := toActivity(pr, dto)

	// Assert
	assert.Nil(t, activity) // No body and not approved/changes requested
}

func TestToActivity_Commit(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()

	dto := TimelineItemDTO{
		Typename:  "PullRequestCommit",
		CreatedAt: now,
	}
	dto.Commit = &struct {
		OID           string    `json:"oid"`
		CommittedDate time.Time `json:"committedDate"`
		Author        *struct {
			User *struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"author"`
	}{
		OID:           "abcdef1234567890",
		CommittedDate: now,
	}
	dto.Commit.Author = &struct {
		User *struct {
			Login string `json:"login"`
		} `json:"user"`
	}{}
	dto.Commit.Author.User = &struct {
		Login string `json:"login"`
	}{Login: "committer"}

	// Act
	activity := toActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeCommit, activity.Type())
	assert.Equal(t, "committer", activity.Author().Login())
	assert.Equal(t, "abcdef1", activity.Body()) // Short SHA
}

func TestToActivity_Comment_NoAuthor_ReturnsNil(t *testing.T) {
	// Arrange
	pr := testutil.NewTestPullRequest(1)

	dto := TimelineItemDTO{
		Typename:  "IssueComment",
		CreatedAt: time.Now(),
		Body:      "Test comment",
		Author:    nil, // No author
	}

	// Act
	activity := toActivity(pr, dto)

	// Assert
	assert.Nil(t, activity)
}

// Helper function to create a valid DTO
func createValidDTO(number int, title string) PullRequestDTO {
	dto := PullRequestDTO{
		Title:     title,
		URL:       "https://github.com/owner/repo/pull/" + string(rune(number)),
		Number:    number,
		CreatedAt: time.Now(),
		IsDraft:   false,
	}
	dto.Repository.NameWithOwner = "owner/repo"
	dto.Author.Login = "testuser"
	return dto
}

// --- Review mapping tests ---

func TestToReviews_ValidReviews(t *testing.T) {
	// Arrange
	now := time.Now()

	dtos := []ReviewDTO{
		{
			State:       "APPROVED",
			SubmittedAt: now,
		},
		{
			State:       "CHANGES_REQUESTED",
			SubmittedAt: now,
		},
	}
	dtos[0].Author.Login = "joe"
	dtos[1].Author.Login = "alice"

	// Act
	reviews := toReviews(dtos)

	// Assert
	assert.Len(t, reviews, 2)
	assert.Equal(t, pullrequest.ReviewStateApproved, reviews["joe"].State())
	assert.Equal(t, pullrequest.ReviewStateChangesRequested, reviews["alice"].State())
}

func TestToReviews_UnknownState_Skipped(t *testing.T) {
	// Arrange
	dto1 := ReviewDTO{
		State:       "APPROVED",
		SubmittedAt: time.Now(),
	}
	dto1.Author.Login = "joe"

	dto2 := ReviewDTO{
		State:       "PENDING",
		SubmittedAt: time.Now(),
	}
	dto2.Author.Login = "bob"

	dtos := []ReviewDTO{dto1, dto2}

	// Act
	reviews := toReviews(dtos)

	// Assert
	assert.Len(t, reviews, 1)
	assert.NotNil(t, reviews["joe"])
}

func TestToReviews_EmptyAuthor_Skipped(t *testing.T) {
	// Arrange
	dtos := []ReviewDTO{
		{
			State:       "APPROVED",
			SubmittedAt: time.Now(),
		},
	}
	// Author.Login is empty by default

	// Act
	reviews := toReviews(dtos)

	// Assert
	assert.Empty(t, reviews)
}

func TestToReviews_Empty(t *testing.T) {
	// Act
	reviews := toReviews(nil)

	// Assert
	assert.Empty(t, reviews)
}

func TestToDomain_WithReviews(t *testing.T) {
	// Arrange
	now := time.Now()

	reviewDTO := ReviewDTO{
		State:       "APPROVED",
		SubmittedAt: now,
	}
	reviewDTO.Author.Login = "joe"

	dto := PullRequestDTO{
		Title:     "Test PR",
		URL:       "https://github.com/owner/repo/pull/1",
		Number:    1,
		CreatedAt: now,
		IsDraft:   false,
		LatestReviews: &LatestReviewsDTO{
			Nodes: []ReviewDTO{reviewDTO},
		},
	}
	dto.Repository.NameWithOwner = "owner/repo"
	dto.Author.Login = "testuser"

	// Act
	pr, err := toDomain(dto)

	// Assert
	require.NoError(t, err)
	// SetReviews is a pure state setter — it never raises domain events.
	reviews := pr.Reviews()
	assert.Len(t, reviews, 1)
	assert.Equal(t, pullrequest.ReviewStateApproved, reviews["joe"].State())
}

// --- GitHub API conversion function tests ---

func TestReviewStateFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected pullrequest.ReviewState
		ok       bool
	}{
		{"APPROVED", pullrequest.ReviewStateApproved, true},
		{"CHANGES_REQUESTED", pullrequest.ReviewStateChangesRequested, true},
		{"COMMENTED", pullrequest.ReviewStateCommented, true},
		{"DISMISSED", pullrequest.ReviewStateDismissed, true},
		{"UNKNOWN", 0, false},
		{"", 0, false},
		{"approved", 0, false}, // Case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			state, ok := reviewStateFromString(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.expected, state)
			}
		})
	}
}

func TestPipelineStatusFromRollup(t *testing.T) {
	tests := []struct {
		state    string
		expected pullrequest.PipelineStatus
	}{
		{"PENDING", pullrequest.PipelineStatusRunning},
		{"IN_PROGRESS", pullrequest.PipelineStatusRunning},
		{"WAITING", pullrequest.PipelineStatusRunning},
		{"QUEUED", pullrequest.PipelineStatusRunning},
		{"SUCCESS", pullrequest.PipelineStatusSuccess},
		{"NEUTRAL", pullrequest.PipelineStatusSuccess},
		{"SKIPPED", pullrequest.PipelineStatusSuccess},
		{"FAILURE", pullrequest.PipelineStatusFailed},
		{"ERROR", pullrequest.PipelineStatusFailed},
		{"CANCELLED", pullrequest.PipelineStatusFailed},
		{"TIMED_OUT", pullrequest.PipelineStatusFailed},
		{"", pullrequest.PipelineStatusUnknown},
		{"SOME_FUTURE_STATE", pullrequest.PipelineStatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			result := pipelineStatusFromRollup(tt.state)
			assert.Equal(t, tt.expected, result)
		})
	}
}
