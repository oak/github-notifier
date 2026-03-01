package pullrequest_test

import (
	"testing"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewDraftFilter_IncludeDraftsTrue(t *testing.T) {
	// Arrange
	filter := pullrequest.NewDraftFilter(true)
	prs := testutil.CreateTestPRs(2, 3) // 2 regular, 3 drafts

	// Act
	result := filter(prs)

	// Assert
	assert.Len(t, result, 5, "Should include all PRs when includeDrafts is true")
	testutil.AssertPRSlicesEqual(t, prs, result)
}

func TestNewDraftFilter_IncludeDraftsFalse(t *testing.T) {
	// Arrange
	filter := pullrequest.NewDraftFilter(false)
	prs := testutil.CreateTestPRs(2, 3) // 2 regular, 3 drafts

	// Act
	result := filter(prs)

	// Assert
	assert.Len(t, result, 2, "Should exclude draft PRs when includeDrafts is false")
	testutil.AssertNoDrafts(t, result)
}

func TestNewDraftFilter_EmptyInput(t *testing.T) {
	// Arrange
	filter := pullrequest.NewDraftFilter(false)
	var prs []*pullrequest.PullRequest

	// Act
	result := filter(prs)

	// Assert
	assert.Empty(t, result, "Should return empty slice for empty input")
}

func TestNewDraftFilter_OnlyDrafts(t *testing.T) {
	// Arrange
	filter := pullrequest.NewDraftFilter(false)
	prs := testutil.CreateTestPRs(0, 3) // 0 regular, 3 drafts

	// Act
	result := filter(prs)

	// Assert
	assert.Empty(t, result, "Should return empty slice when all PRs are drafts and includeDrafts is false")
}

func TestNewDraftFilter_OnlyRegular(t *testing.T) {
	// Arrange
	filter := pullrequest.NewDraftFilter(false)
	prs := testutil.CreateTestPRs(3, 0) // 3 regular, 0 drafts

	// Act
	result := filter(prs)

	// Assert
	assert.Len(t, result, 3, "Should return all PRs when none are drafts")
	testutil.AssertPRSlicesEqual(t, prs, result)
}

func TestNewDraftFilter_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		includeDrafts    bool
		regularCount     int
		draftCount       int
		expectedCount    int
		shouldHaveDrafts bool
	}{
		{
			name:             "include drafts with mixed PRs",
			includeDrafts:    true,
			regularCount:     2,
			draftCount:       3,
			expectedCount:    5,
			shouldHaveDrafts: true,
		},
		{
			name:             "exclude drafts with mixed PRs",
			includeDrafts:    false,
			regularCount:     2,
			draftCount:       3,
			expectedCount:    2,
			shouldHaveDrafts: false,
		},
		{
			name:             "include drafts with only regular PRs",
			includeDrafts:    true,
			regularCount:     5,
			draftCount:       0,
			expectedCount:    5,
			shouldHaveDrafts: false,
		},
		{
			name:             "exclude drafts with only regular PRs",
			includeDrafts:    false,
			regularCount:     5,
			draftCount:       0,
			expectedCount:    5,
			shouldHaveDrafts: false,
		},
		{
			name:             "include drafts with only draft PRs",
			includeDrafts:    true,
			regularCount:     0,
			draftCount:       5,
			expectedCount:    5,
			shouldHaveDrafts: true,
		},
		{
			name:             "exclude drafts with only draft PRs",
			includeDrafts:    false,
			regularCount:     0,
			draftCount:       5,
			expectedCount:    0,
			shouldHaveDrafts: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			filter := pullrequest.NewDraftFilter(tt.includeDrafts)
			prs := testutil.CreateTestPRs(tt.regularCount, tt.draftCount)

			// Act
			result := filter(prs)

			// Assert
			assert.Len(t, result, tt.expectedCount, "Result should have expected count")

			if tt.shouldHaveDrafts {
				hasDraft := false
				for _, pr := range result {
					if pr.IsDraft() {
						hasDraft = true
						break
					}
				}
				assert.True(t, hasDraft, "Result should contain at least one draft PR")
			} else {
				testutil.AssertNoDrafts(t, result)
			}
		})
	}
}
