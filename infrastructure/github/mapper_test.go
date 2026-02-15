package github_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/github"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapper_ToDomain_ValidDTO(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	now := time.Now()

	dto := github.PullRequestDTO{
		Title:     "Test PR",
		URL:       "https://github.com/owner/repo/pull/1",
		Number:    1,
		CreatedAt: now,
		IsDraft:   false,
	}
	dto.Repository.NameWithOwner = "owner/repo"
	dto.Author.Login = "testuser"

	// Act
	pr, err := mapper.ToDomain(dto)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Test PR", pr.Title())
	assert.Equal(t, 1, pr.Number())
	assert.Equal(t, "https://github.com/owner/repo/pull/1", pr.URL())
	assert.Equal(t, "owner/repo", pr.Repository().NameWithOwner())
	assert.Equal(t, "testuser", pr.Author().Login())
	assert.False(t, pr.IsDraft())
}

func TestMapper_ToDomain_InvalidRepository(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()

	dto := github.PullRequestDTO{
		Title:  "Test PR",
		URL:    "https://github.com/owner/repo/pull/1",
		Number: 1,
	}
	dto.Repository.NameWithOwner = "" // Invalid
	dto.Author.Login = "testuser"

	// Act
	pr, err := mapper.ToDomain(dto)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "failed to create repository")
}

func TestMapper_ToDomain_InvalidAuthor(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()

	dto := github.PullRequestDTO{
		Title:  "Test PR",
		URL:    "https://github.com/owner/repo/pull/1",
		Number: 1,
	}
	dto.Repository.NameWithOwner = "owner/repo"
	dto.Author.Login = "" // Invalid

	// Act
	pr, err := mapper.ToDomain(dto)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "failed to create author")
}

func TestMapper_ToDomainList_ValidDTOs(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()

	dto1 := createValidDTO(1, "PR 1")
	dto2 := createValidDTO(2, "PR 2")

	// Act
	prs, err := mapper.ToDomainList([]github.PullRequestDTO{dto1, dto2})

	// Assert
	require.NoError(t, err)
	assert.Len(t, prs, 2)
	assert.Equal(t, 1, prs[0].Number())
	assert.Equal(t, 2, prs[1].Number())
}

func TestMapper_ToDomainList_SkipsInvalidDTOs(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()

	validDTO := createValidDTO(1, "Valid PR")
	invalidDTO := github.PullRequestDTO{} // Missing required fields

	// Act
	prs, err := mapper.ToDomainList([]github.PullRequestDTO{validDTO, invalidDTO})

	// Assert
	require.NoError(t, err)
	assert.Len(t, prs, 1) // Only valid PR
	assert.Equal(t, 1, prs[0].Number())
}

func TestMapper_ToActivity_Comment(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()

	dto := github.TimelineItemDTO{
		Typename:  "IssueComment",
		CreatedAt: now,
		Body:      "Test comment",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "commenter"}

	// Act
	activity := mapper.ToActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeComment, activity.Type())
	assert.Equal(t, "commenter", activity.Author().Login())
	assert.Equal(t, "Test comment", activity.Body())
}

func TestMapper_ToActivity_Review_WithBody(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	pr := testutil.NewTestPullRequest(1)

	dto := github.TimelineItemDTO{
		Typename:  "PullRequestReview",
		CreatedAt: time.Now(),
		Body:      "Looks good!",
		State:     "COMMENTED",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "reviewer"}

	// Act
	activity := mapper.ToActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeReview, activity.Type())
	assert.Equal(t, "reviewer", activity.Author().Login())
}

func TestMapper_ToActivity_Review_Approved(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	pr := testutil.NewTestPullRequest(1)

	dto := github.TimelineItemDTO{
		Typename:  "PullRequestReview",
		CreatedAt: time.Now(),
		Body:      "",
		State:     "APPROVED",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "reviewer"}

	// Act
	activity := mapper.ToActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeReview, activity.Type())
}

func TestMapper_ToActivity_Review_ChangesRequested(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	pr := testutil.NewTestPullRequest(1)

	dto := github.TimelineItemDTO{
		Typename:  "PullRequestReview",
		CreatedAt: time.Now(),
		Body:      "",
		State:     "CHANGES_REQUESTED",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "reviewer"}

	// Act
	activity := mapper.ToActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeReview, activity.Type())
}

func TestMapper_ToActivity_Review_EmptyBodyNoState_ReturnsNil(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	pr := testutil.NewTestPullRequest(1)

	dto := github.TimelineItemDTO{
		Typename:  "PullRequestReview",
		CreatedAt: time.Now(),
		Body:      "",
		State:     "COMMENTED",
	}
	dto.Author = &struct {
		Login string `json:"login"`
	}{Login: "reviewer"}

	// Act
	activity := mapper.ToActivity(pr, dto)

	// Assert
	assert.Nil(t, activity) // No body and not approved/changes requested
}

func TestMapper_ToActivity_Commit(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	pr := testutil.NewTestPullRequest(1)
	now := time.Now()

	dto := github.TimelineItemDTO{
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
	activity := mapper.ToActivity(pr, dto)

	// Assert
	require.NotNil(t, activity)
	assert.Equal(t, pullrequest.ActivityTypeCommit, activity.Type())
	assert.Equal(t, "committer", activity.Author().Login())
	assert.Equal(t, "abcdef1", activity.Body()) // Short SHA
}

func TestMapper_ToActivity_Comment_NoAuthor_ReturnsNil(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	pr := testutil.NewTestPullRequest(1)

	dto := github.TimelineItemDTO{
		Typename:  "IssueComment",
		CreatedAt: time.Now(),
		Body:      "Test comment",
		Author:    nil, // No author
	}

	// Act
	activity := mapper.ToActivity(pr, dto)

	// Assert
	assert.Nil(t, activity)
}

// Helper function to create a valid DTO
func createValidDTO(number int, title string) github.PullRequestDTO {
	dto := github.PullRequestDTO{
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

func TestMapper_ToReviews_ValidReviews(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	now := time.Now()

	dtos := []github.ReviewDTO{
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
	reviews := mapper.ToReviews(dtos)

	// Assert
	assert.Len(t, reviews, 2)
	assert.Equal(t, pullrequest.ReviewStateApproved, reviews["joe"].State())
	assert.Equal(t, pullrequest.ReviewStateChangesRequested, reviews["alice"].State())
}

func TestMapper_ToReviews_UnknownState_Skipped(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()

	dto1 := github.ReviewDTO{
		State:       "APPROVED",
		SubmittedAt: time.Now(),
	}
	dto1.Author.Login = "joe"

	dto2 := github.ReviewDTO{
		State:       "PENDING",
		SubmittedAt: time.Now(),
	}
	dto2.Author.Login = "bob"

	dtos := []github.ReviewDTO{dto1, dto2}

	// Act
	reviews := mapper.ToReviews(dtos)

	// Assert
	assert.Len(t, reviews, 1)
	assert.NotNil(t, reviews["joe"])
}

func TestMapper_ToReviews_EmptyAuthor_Skipped(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()

	dtos := []github.ReviewDTO{
		{
			State:       "APPROVED",
			SubmittedAt: time.Now(),
		},
	}
	// Author.Login is empty by default

	// Act
	reviews := mapper.ToReviews(dtos)

	// Assert
	assert.Empty(t, reviews)
}

func TestMapper_ToReviews_Empty(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()

	// Act
	reviews := mapper.ToReviews(nil)

	// Assert
	assert.Empty(t, reviews)
}

func TestMapper_ToDomain_WithReviews(t *testing.T) {
	// Arrange
	mapper := github.NewMapper()
	now := time.Now()

	reviewDTO := github.ReviewDTO{
		State:       "APPROVED",
		SubmittedAt: now,
	}
	reviewDTO.Author.Login = "joe"

	dto := github.PullRequestDTO{
		Title:     "Test PR",
		URL:       "https://github.com/owner/repo/pull/1",
		Number:    1,
		CreatedAt: now,
		IsDraft:   false,
		LatestReviews: &github.LatestReviewsDTO{
			Nodes: []github.ReviewDTO{reviewDTO},
		},
	}
	dto.Repository.NameWithOwner = "owner/repo"
	dto.Author.Login = "testuser"

	// Act
	pr, err := mapper.ToDomain(dto)

	// Assert
	require.NoError(t, err)
	reviews := pr.Reviews()
	assert.Len(t, reviews, 1)
	assert.Equal(t, pullrequest.ReviewStateApproved, reviews["joe"].State())

	// SetInitialReviews should not raise events
	events := pr.CollectEvents()
	assert.Empty(t, events)
}
