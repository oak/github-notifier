package ui

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestFormatReviewSummaryForMenu_SingleApproval(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Joe": pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	assert.Equal(t, "(✅ Joe)", FormatReviewSummaryForMenu(summary))
}

func TestFormatReviewSummaryForMenu_SingleChangesRequested(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Alice": pullrequest.NewReview(testutil.NewTestAuthor("Alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	assert.Equal(t, "(❌ Alice)", FormatReviewSummaryForMenu(summary))
}

func TestFormatReviewSummaryForMenu_MixedStates(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Joe":   pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
		"Mike":  pullrequest.NewReview(testutil.NewTestAuthor("Mike"), pullrequest.ReviewStateApproved, time.Now()),
		"Alice": pullrequest.NewReview(testutil.NewTestAuthor("Alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	result := FormatReviewSummaryForMenu(summary)
	// Approved comes first, then changes requested. Names are sorted within each group.
	assert.Equal(t, "(✅ Joe, Mike | ❌ Alice)", result)
}

func TestFormatReviewSummaryForMenu_CommentedOnly(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Bob": pullrequest.NewReview(testutil.NewTestAuthor("Bob"), pullrequest.ReviewStateCommented, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	assert.Equal(t, "(💬 Bob)", FormatReviewSummaryForMenu(summary))
}

func TestFormatReviewSummaryForMenu_DismissedNotShown(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Dan": pullrequest.NewReview(testutil.NewTestAuthor("Dan"), pullrequest.ReviewStateDismissed, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	// Dismissed reviews are not shown in the compact menu view
	assert.Equal(t, "", FormatReviewSummaryForMenu(summary))
}

func TestFormatReviewSummaryForMenu_Empty(t *testing.T) {
	summary := pullrequest.NewReviewSummary(make(map[string]*pullrequest.Review))
	assert.Equal(t, "", FormatReviewSummaryForMenu(summary))
}

func TestFormatReviewSummaryForMenu_AllStates(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Joe":   pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
		"Bob":   pullrequest.NewReview(testutil.NewTestAuthor("Bob"), pullrequest.ReviewStateCommented, time.Now()),
		"Alice": pullrequest.NewReview(testutil.NewTestAuthor("Alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	result := FormatReviewSummaryForMenu(summary)
	// Order: approved | commented | changes_requested
	assert.Equal(t, "(✅ Joe | 💬 Bob | ❌ Alice)", result)
}
