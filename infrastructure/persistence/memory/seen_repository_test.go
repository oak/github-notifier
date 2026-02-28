package memory_test

import (
	"testing"

	"github.com/oak3/github-notifier/infrastructure/persistence/memory"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeenRepository_MarkAsSeen(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()
	pr := testutil.NewTestPullRequest(1)

	// Act
	err := repo.MarkAsSeen(pr.Identifier())

	// Assert
	require.NoError(t, err)
	assert.True(t, repo.HasBeenSeen(pr.Identifier()))
}

func TestSeenRepository_HasBeenSeen_NotSeen(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()
	pr := testutil.NewTestPullRequest(1)

	// Act & Assert
	assert.False(t, repo.HasBeenSeen(pr.Identifier()))
}

func TestSeenRepository_UnmarkAsSeen(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()
	pr := testutil.NewTestPullRequest(1)

	// Mark as seen first
	err := repo.MarkAsSeen(pr.Identifier())
	require.NoError(t, err)
	assert.True(t, repo.HasBeenSeen(pr.Identifier()))

	// Act - unmark
	err = repo.UnmarkAsSeen(pr.Identifier())

	// Assert
	require.NoError(t, err)
	assert.False(t, repo.HasBeenSeen(pr.Identifier()))
}

func TestSeenRepository_IsEmpty_Initially(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()

	// Act & Assert
	assert.True(t, repo.IsEmpty())
}

func TestSeenRepository_IsEmpty_AfterMarking(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()
	pr := testutil.NewTestPullRequest(1)

	// Act
	err := repo.MarkAsSeen(pr.Identifier())
	require.NoError(t, err)

	// Assert
	assert.False(t, repo.IsEmpty())
}

func TestSeenRepository_Clear(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()
	pr1 := testutil.NewTestPullRequest(1)
	pr2 := testutil.NewTestPullRequest(2)

	repo.MarkAsSeen(pr1.Identifier())
	repo.MarkAsSeen(pr2.Identifier())
	assert.False(t, repo.IsEmpty())

	// Act
	err := repo.Clear()

	// Assert
	require.NoError(t, err)
	assert.True(t, repo.IsEmpty())
	assert.False(t, repo.HasBeenSeen(pr1.Identifier()))
	assert.False(t, repo.HasBeenSeen(pr2.Identifier()))
}

func TestSeenRepository_MultiplePRs(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()
	pr1 := testutil.NewTestPullRequest(1)
	pr2 := testutil.NewTestPullRequest(2)
	pr3 := testutil.NewTestPullRequest(3)

	// Act - mark pr1 and pr2 as seen
	repo.MarkAsSeen(pr1.Identifier())
	repo.MarkAsSeen(pr2.Identifier())

	// Assert
	assert.True(t, repo.HasBeenSeen(pr1.Identifier()))
	assert.True(t, repo.HasBeenSeen(pr2.Identifier()))
	assert.False(t, repo.HasBeenSeen(pr3.Identifier())) // pr3 was never marked as seen
}

func TestSeenRepository_MarkSamePRTwice(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()
	pr := testutil.NewTestPullRequest(1)

	// Act - mark twice
	err1 := repo.MarkAsSeen(pr.Identifier())
	err2 := repo.MarkAsSeen(pr.Identifier())

	// Assert - should not error and should still be marked
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.True(t, repo.HasBeenSeen(pr.Identifier()))
}

func TestSeenRepository_UnmarkNotSeenPR(t *testing.T) {
	// Arrange
	repo := memory.NewSeenPullRequestRepository()
	pr := testutil.NewTestPullRequest(1)

	// Act - unmark a PR that was never marked
	err := repo.UnmarkAsSeen(pr.Identifier())

	// Assert - should not error
	require.NoError(t, err)
	assert.False(t, repo.HasBeenSeen(pr.Identifier()))
}
