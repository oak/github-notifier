package pullrequest_test

import (
	"strings"
	"testing"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PRIdentifier tests

func TestNewPRIdentifier_ValidInput(t *testing.T) {
	// Arrange
	url := "https://github.com/owner/repo/pull/123"
	number := 123

	// Act
	id, err := pullrequest.NewPRIdentifier(url, number)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, url, id.URL())
	assert.Equal(t, number, id.Number())
}

func TestNewPRIdentifier_EmptyURL_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewPRIdentifier("", 123)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "URL cannot be empty")
}

func TestNewPRIdentifier_ZeroNumber_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/0", 0)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "number must be positive")
}

func TestNewPRIdentifier_NegativeNumber_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/-1", -1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "number must be positive")
}

func TestPRIdentifier_Equals(t *testing.T) {
	// Arrange
	id1, _ := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/123", 123)
	id2, _ := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/123", 123)
	id3, _ := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/456", 456)

	// Act & Assert
	assert.True(t, id1.Equals(id2))
	assert.False(t, id1.Equals(id3))
}

func TestPRIdentifier_String(t *testing.T) {
	// Arrange
	id, _ := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/123", 123)

	// Act
	str := id.String()

	// Assert
	assert.Contains(t, str, "123")
	assert.Contains(t, str, "https://github.com/owner/repo/pull/123")
}

// RepositoryInfo tests

func TestNewRepository_ValidInput(t *testing.T) {
	// Act
	repo, err := pullrequest.NewRepository("owner/repo")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "owner/repo", repo.NameWithOwner())
	assert.Equal(t, "owner", repo.Owner())
	assert.Equal(t, "repo", repo.Name())
}

func TestNewRepository_EmptyString_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewRepository("")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository name cannot be empty")
}

func TestNewRepository_MissingOwner_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewRepository("/repo")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "owner and name cannot be empty")
}

func TestNewRepository_MissingRepo_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewRepository("owner/")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "owner and name cannot be empty")
}

func TestNewRepository_NoSlash_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewRepository("ownerrepo")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repository format")
}

func TestNewRepository_MultipleSlashes_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewRepository("owner/repo/extra")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repository format")
}

func TestRepository_Equals(t *testing.T) {
	// Arrange
	repo1, _ := pullrequest.NewRepository("owner/repo")
	repo2, _ := pullrequest.NewRepository("owner/repo")
	repo3, _ := pullrequest.NewRepository("other/repo")

	// Act & Assert
	assert.True(t, repo1.Equals(repo2))
	assert.False(t, repo1.Equals(repo3))
}

func TestRepository_String(t *testing.T) {
	// Arrange
	repo, _ := pullrequest.NewRepository("owner/repo")

	// Act
	str := repo.String()

	// Assert
	assert.Equal(t, "owner/repo", str)
}

func TestRepository_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectError   bool
		expectedOwner string
		expectedName  string
	}{
		{
			name:          "valid repository",
			input:         "owner/repo",
			expectError:   false,
			expectedOwner: "owner",
			expectedName:  "repo",
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "no slash",
			input:       "ownerrepo",
			expectError: true,
		},
		{
			name:        "multiple slashes",
			input:       "owner/repo/extra",
			expectError: true,
		},
		{
			name:        "empty owner",
			input:       "/repo",
			expectError: true,
		},
		{
			name:        "empty repo",
			input:       "owner/",
			expectError: true,
		},
		{
			name:          "valid with hyphen",
			input:         "my-owner/my-repo",
			expectError:   false,
			expectedOwner: "my-owner",
			expectedName:  "my-repo",
		},
		{
			name:          "valid with underscore",
			input:         "my_owner/my_repo",
			expectError:   false,
			expectedOwner: "my_owner",
			expectedName:  "my_repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			repo, err := pullrequest.NewRepository(tt.input)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOwner, repo.Owner())
				assert.Equal(t, tt.expectedName, repo.Name())
			}
		})
	}
}

// Author tests

func TestNewAuthor_ValidInput(t *testing.T) {
	// Act
	author, err := pullrequest.NewAuthor("testuser")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "testuser", author.Login())
}

func TestNewAuthor_EmptyLogin_ReturnsError(t *testing.T) {
	// Act
	_, err := pullrequest.NewAuthor("")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "login cannot be empty")
}

func TestNewAuthor_TooLong_ReturnsError(t *testing.T) {
	// Arrange - GitHub username max length is 39
	longLogin := strings.Repeat("a", 40)

	// Act
	_, err := pullrequest.NewAuthor(longLogin)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot exceed 39 characters")
}

func TestNewAuthor_MaxLength_Valid(t *testing.T) {
	// Arrange - Exactly 39 characters
	maxLogin := strings.Repeat("a", 39)

	// Act
	author, err := pullrequest.NewAuthor(maxLogin)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, maxLogin, author.Login())
}

func TestAuthor_Equals(t *testing.T) {
	// Arrange
	author1, _ := pullrequest.NewAuthor("user1")
	author2, _ := pullrequest.NewAuthor("user1")
	author3, _ := pullrequest.NewAuthor("user2")

	// Act & Assert
	assert.True(t, author1.Equals(author2))
	assert.False(t, author1.Equals(author3))
}

func TestAuthor_String(t *testing.T) {
	// Arrange
	author, _ := pullrequest.NewAuthor("testuser")

	// Act
	str := author.String()

	// Assert
	assert.Equal(t, "testuser", str)
}

func TestAuthor_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		login       string
		expectError bool
	}{
		{
			name:        "valid short login",
			login:       "user",
			expectError: false,
		},
		{
			name:        "valid with numbers",
			login:       "user123",
			expectError: false,
		},
		{
			name:        "valid with hyphen",
			login:       "my-user",
			expectError: false,
		},
		{
			name:        "empty login",
			login:       "",
			expectError: true,
		},
		{
			name:        "exactly 39 characters",
			login:       strings.Repeat("a", 39),
			expectError: false,
		},
		{
			name:        "40 characters - too long",
			login:       strings.Repeat("a", 40),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			author, err := pullrequest.NewAuthor(tt.login)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.login, author.Login())
			}
		})
	}
}

// PRStatus tests

func TestPRStatus_String(t *testing.T) {
	tests := []struct {
		status   pullrequest.PRStatus
		expected string
	}{
		{pullrequest.StatusOpen, "open"},
		{pullrequest.StatusMerged, "merged"},
		{pullrequest.StatusClosed, "closed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestPRStatus_IsOpen(t *testing.T) {
	tests := []struct {
		status   pullrequest.PRStatus
		expected bool
	}{
		{pullrequest.StatusOpen, true},
		{pullrequest.StatusMerged, false},
		{pullrequest.StatusClosed, false},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsOpen())
		})
	}
}

// PipelineStatus tests

func TestPipelineStatus_String(t *testing.T) {
	tests := []struct {
		status   pullrequest.PipelineStatus
		expected string
	}{
		{pullrequest.PipelineStatusUnknown, "unknown"},
		{pullrequest.PipelineStatusRunning, "running"},
		{pullrequest.PipelineStatusSuccess, "success"},
		{pullrequest.PipelineStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestPipelineStatus_Emoji(t *testing.T) {
	tests := []struct {
		status   pullrequest.PipelineStatus
		expected string
	}{
		{pullrequest.PipelineStatusUnknown, "❓"},
		{pullrequest.PipelineStatusRunning, "🟡"},
		{pullrequest.PipelineStatusSuccess, "🟢"},
		{pullrequest.PipelineStatusFailed, "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.Emoji())
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
			result := pullrequest.PipelineStatusFromRollup(tt.state)
			assert.Equal(t, tt.expected, result)
		})
	}
}
