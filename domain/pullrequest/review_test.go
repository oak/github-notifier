package pullrequest_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

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

func TestReviewSummary_ReviewersByState(t *testing.T) {
	reviews := map[string]*pullrequest.Review{
		"Joe":   pullrequest.NewReview(testutil.NewTestAuthor("Joe"), pullrequest.ReviewStateApproved, time.Now()),
		"Alice": pullrequest.NewReview(testutil.NewTestAuthor("Alice"), pullrequest.ReviewStateChangesRequested, time.Now()),
		"Bob":   pullrequest.NewReview(testutil.NewTestAuthor("Bob"), pullrequest.ReviewStateApproved, time.Now()),
	}
	summary := pullrequest.NewReviewSummary(reviews)

	approved := summary.ReviewersByState(pullrequest.ReviewStateApproved)
	assert.Len(t, approved, 2)
	assert.Contains(t, approved, "Joe")
	assert.Contains(t, approved, "Bob")

	changesRequested := summary.ReviewersByState(pullrequest.ReviewStateChangesRequested)
	assert.Len(t, changesRequested, 1)
	assert.Contains(t, changesRequested, "Alice")

	commented := summary.ReviewersByState(pullrequest.ReviewStateCommented)
	assert.Empty(t, commented)
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
