package pullrequest_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

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
			state, ok := pullrequest.ReviewStateFromString(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.expected, state)
			}
		})
	}
}

func TestReviewState_String(t *testing.T) {
	assert.Equal(t, "approved", pullrequest.ReviewStateApproved.String())
	assert.Equal(t, "changes_requested", pullrequest.ReviewStateChangesRequested.String())
	assert.Equal(t, "commented", pullrequest.ReviewStateCommented.String())
	assert.Equal(t, "dismissed", pullrequest.ReviewStateDismissed.String())
}

func TestReviewState_Emoji(t *testing.T) {
	assert.Equal(t, "✅", pullrequest.ReviewStateApproved.Emoji())
	assert.Equal(t, "❌", pullrequest.ReviewStateChangesRequested.Emoji())
	assert.Equal(t, "💬", pullrequest.ReviewStateCommented.Emoji())
	assert.Equal(t, "🚫", pullrequest.ReviewStateDismissed.Emoji())
}

func TestReviewState_Label(t *testing.T) {
	assert.Equal(t, "approved", pullrequest.ReviewStateApproved.Label())
	assert.Equal(t, "requested changes", pullrequest.ReviewStateChangesRequested.Label())
	assert.Equal(t, "commented", pullrequest.ReviewStateCommented.Label())
	assert.Equal(t, "dismissed", pullrequest.ReviewStateDismissed.Label())
}

func TestNewReview(t *testing.T) {
	author := testutil.NewTestAuthor("joe")
	now := time.Now()

	review := pullrequest.NewReview(author, pullrequest.ReviewStateApproved, now)

	assert.Equal(t, "joe", review.Reviewer().Login())
	assert.Equal(t, pullrequest.ReviewStateApproved, review.State())
	assert.True(t, review.SubmittedAt().Equal(now))
}

func TestReviewSummary_IsEmpty(t *testing.T) {
	summary := pullrequest.NewReviewSummary(make(map[string]*pullrequest.Review))
	assert.True(t, summary.IsEmpty())

	reviews := map[string]*pullrequest.Review{
		"joe": pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, time.Now()),
	}
	summary = pullrequest.NewReviewSummary(reviews)
	assert.False(t, summary.IsEmpty())
}

func TestReviewSummary_FormatForMenu_SingleApproval(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Joe": pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	assert.Equal(t, "(✅ Joe)", summary.FormatForMenu())
}

func TestReviewSummary_FormatForMenu_SingleChangesRequested(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Alice": pullrequest.NewReview(testutil.NewTestAuthor("Alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	assert.Equal(t, "(❌ Alice)", summary.FormatForMenu())
}

func TestReviewSummary_FormatForMenu_MixedStates(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Joe":   pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
		"Mike":  pullrequest.NewReview(testutil.NewTestAuthor("Mike"), pullrequest.ReviewStateApproved, time.Now()),
		"Alice": pullrequest.NewReview(testutil.NewTestAuthor("Alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	result := summary.FormatForMenu()
	// Approved comes first, then changes requested. Names are sorted within each group.
	assert.Equal(t, "(✅ Joe, Mike | ❌ Alice)", result)
}

func TestReviewSummary_FormatForMenu_CommentedOnly(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Bob": pullrequest.NewReview(testutil.NewTestAuthor("Bob"), pullrequest.ReviewStateCommented, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	assert.Equal(t, "(💬 Bob)", summary.FormatForMenu())
}

func TestReviewSummary_FormatForMenu_DismissedNotShown(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Dan": pullrequest.NewReview(testutil.NewTestAuthor("Dan"), pullrequest.ReviewStateDismissed, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	// Dismissed reviews are not shown in the compact menu view
	assert.Equal(t, "", summary.FormatForMenu())
}

func TestReviewSummary_FormatForMenu_Empty(t *testing.T) {
	summary := pullrequest.NewReviewSummary(make(map[string]*pullrequest.Review))
	assert.Equal(t, "", summary.FormatForMenu())
}

func TestReviewSummary_FormatForMenu_AllStates(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Joe":   pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
		"Bob":   pullrequest.NewReview(testutil.NewTestAuthor("Bob"), pullrequest.ReviewStateCommented, time.Now()),
		"Alice": pullrequest.NewReview(testutil.NewTestAuthor("Alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	result := summary.FormatForMenu()
	// Order: approved | commented | changes_requested
	assert.Equal(t, "(✅ Joe | 💬 Bob | ❌ Alice)", result)
}

func TestReviewSummary_Reviews_ReturnsCopy(t *testing.T) {
	original := map[string]*pullrequest.Review{
		"Joe": pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(original)

	copy := summary.Reviews()
	assert.Len(t, copy, 1)

	// Modifying the copy should not affect the original
	delete(copy, "Joe")
	assert.Len(t, summary.Reviews(), 1)
}
