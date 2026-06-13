package events

// White-box tests for the pure accumulateEvent fold function.
// No goroutines, no timers, no mocks — just plain table-driven unit tests.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oak/github-notifier/application/port"
	"github.com/oak/github-notifier/domain/pullrequest"
	"github.com/oak/github-notifier/internal/testutil"
)

// newBatch returns an empty batch, mirroring what NewNotificationAggregator initialises.
func newBatch() map[string]*port.PRNotificationData {
	return make(map[string]*port.PRNotificationData)
}

// ── NewPullRequestDetected ────────────────────────────────────────────────────

func TestAccumulateEvent_NewPRDetected_SetsIsNew(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	batch := accumulateEvent(newBatch(), &event, "")

	require.Len(t, batch, 1)
	notif := batch[pr.URL()]
	require.NotNil(t, notif)
	assert.True(t, notif.IsNew)
	assert.Equal(t, pr, notif.PullRequest)
}

func TestAccumulateEvent_NewPRDetected_Idempotent(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewNewPullRequestDetected(pr)

	batch := newBatch()
	batch = accumulateEvent(batch, &event, "")
	batch = accumulateEvent(batch, &event, "") // second call — must not duplicate

	require.Len(t, batch, 1, "batch must not grow on repeated NewPRDetected for the same PR")
	assert.True(t, batch[pr.URL()].IsNew)
}

// ── ActivityDetected ─────────────────────────────────────────────────────────

func TestAccumulateEvent_ActivityDetected_AddsActivity(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
		testutil.WithActivityAuthor("alice"),
	)
	event := pullrequest.NewActivityDetected(pr, activity)

	batch := accumulateEvent(newBatch(), &event, "")

	require.Len(t, batch, 1)
	notif := batch[pr.URL()]
	require.Len(t, notif.Activities, 1)
	assert.Equal(t, pullrequest.ActivityTypeComment, notif.Activities[0].Type)
	assert.Equal(t, 1, notif.Activities[0].Count)
}

func TestAccumulateEvent_ActivityDetected_AccumulatesCountForSameType(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	mkEvent := func() pullrequest.ActivityDetected {
		a := testutil.NewTestActivity(
			pullrequest.ActivityTypeComment,
			time.Now(),
			testutil.WithActivityPR(pr.URL(), pr.Number()),
			testutil.WithActivityAuthor("alice"),
		)
		return pullrequest.NewActivityDetected(pr, a)
	}

	batch := newBatch()
	e1 := mkEvent()
	e2 := mkEvent()
	batch = accumulateEvent(batch, &e1, "")
	batch = accumulateEvent(batch, &e2, "")

	notif := batch[pr.URL()]
	require.Len(t, notif.Activities, 1, "same activity type must not create a new entry")
	assert.Equal(t, 2, notif.Activities[0].Count)
}

func TestAccumulateEvent_ActivityDetected_SelfFilteredOut(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
		testutil.WithActivityAuthor("me"),
	)
	event := pullrequest.NewActivityDetected(pr, activity)

	batch := accumulateEvent(newBatch(), &event, "me")

	assert.Empty(t, batch, "self-authored activity must be filtered out")
}

func TestAccumulateEvent_ActivityDetected_NoFilterWhenUserEmpty(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
		testutil.WithActivityAuthor("alice"),
	)
	event := pullrequest.NewActivityDetected(pr, activity)

	// Empty authenticatedUser means no filtering should occur.
	batch := accumulateEvent(newBatch(), &event, "")

	assert.Len(t, batch, 1)
}

// ── ReviewStateChanged ────────────────────────────────────────────────────────

func TestAccumulateEvent_ReviewStateChanged_AddsReviewChange(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	reviewer, err := pullrequest.NewAuthor("bob")
	require.NoError(t, err)
	event := pullrequest.NewReviewStateChanged(pr, reviewer, pullrequest.ReviewStateApproved)

	batch := accumulateEvent(newBatch(), &event, "")

	notif := batch[pr.URL()]
	require.NotNil(t, notif)
	require.Len(t, notif.ReviewChanges, 1)
	assert.Equal(t, "bob", notif.ReviewChanges[0].Reviewer)
	assert.Equal(t, pullrequest.ReviewStateApproved, notif.ReviewChanges[0].State)
}

func TestAccumulateEvent_ReviewStateChanged_SelfFilteredOut(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	reviewer, err := pullrequest.NewAuthor("me")
	require.NoError(t, err)
	event := pullrequest.NewReviewStateChanged(pr, reviewer, pullrequest.ReviewStateApproved)

	batch := accumulateEvent(newBatch(), &event, "me")

	assert.Empty(t, batch, "self-authored review change must be filtered out")
}

func TestAccumulateEvent_ReviewStateChanged_MultipleReviewers(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	bob, _ := pullrequest.NewAuthor("bob")
	carol, _ := pullrequest.NewAuthor("carol")

	e1 := pullrequest.NewReviewStateChanged(pr, bob, pullrequest.ReviewStateApproved)
	e2 := pullrequest.NewReviewStateChanged(pr, carol, pullrequest.ReviewStateChangesRequested)

	batch := newBatch()
	batch = accumulateEvent(batch, &e1, "")
	batch = accumulateEvent(batch, &e2, "")

	notif := batch[pr.URL()]
	require.Len(t, notif.ReviewChanges, 2)
}

// ── Merged ────────────────────────────────────────────────────────────────────

func TestAccumulateEvent_Merged_AddsStatusChange(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewMerged(pr)

	batch := accumulateEvent(newBatch(), &event, "")

	notif := batch[pr.URL()]
	require.NotNil(t, notif)
	require.Len(t, notif.StatusChanges, 1)
	assert.Equal(t, pullrequest.StatusChangeMerged, notif.StatusChanges[0].EventType)
}

func TestAccumulateEvent_Merged_Idempotent(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	e1 := pullrequest.NewMerged(pr)
	e2 := pullrequest.NewMerged(pr)

	batch := newBatch()
	batch = accumulateEvent(batch, &e1, "")
	batch = accumulateEvent(batch, &e2, "")

	// Both status-change entries should be recorded (behaviour is append, not deduplicate)
	notif := batch[pr.URL()]
	assert.Len(t, notif.StatusChanges, 2)
}

// ── Closed ────────────────────────────────────────────────────────────────────

func TestAccumulateEvent_Closed_AddsStatusChange(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewClosed(pr)

	batch := accumulateEvent(newBatch(), &event, "")

	notif := batch[pr.URL()]
	require.NotNil(t, notif)
	require.Len(t, notif.StatusChanges, 1)
	assert.Equal(t, pullrequest.StatusChangeClosed, notif.StatusChanges[0].EventType)
}

// ── PipelineStatusChanged ─────────────────────────────────────────────────────

func TestAccumulateEvent_PipelineStatusChanged_RecordsTransition(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewPipelineStatusChanged(pr, pullrequest.PipelineStatusUnknown, pullrequest.PipelineStatusFailed)

	batch := accumulateEvent(newBatch(), &event, "")

	notif := batch[pr.URL()]
	require.NotNil(t, notif)
	require.NotNil(t, notif.PipelineChange)
	assert.Equal(t, pullrequest.PipelineStatusUnknown, notif.PipelineChange.OldStatus)
	assert.Equal(t, pullrequest.PipelineStatusFailed, notif.PipelineChange.NewStatus)
}

func TestAccumulateEvent_PipelineStatusChanged_PreservesOldStatusOnUpdate(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	e1 := pullrequest.NewPipelineStatusChanged(pr, pullrequest.PipelineStatusUnknown, pullrequest.PipelineStatusRunning)
	e2 := pullrequest.NewPipelineStatusChanged(pr, pullrequest.PipelineStatusRunning, pullrequest.PipelineStatusSuccess)

	batch := newBatch()
	batch = accumulateEvent(batch, &e1, "")
	batch = accumulateEvent(batch, &e2, "")

	notif := batch[pr.URL()]
	require.NotNil(t, notif.PipelineChange)
	// OldStatus must stay from the first event; only NewStatus advances.
	assert.Equal(t, pullrequest.PipelineStatusUnknown, notif.PipelineChange.OldStatus)
	assert.Equal(t, pullrequest.PipelineStatusSuccess, notif.PipelineChange.NewStatus)
}

// ── Cross-event grouping ──────────────────────────────────────────────────────

func TestAccumulateEvent_MultipleEventTypes_GroupedUnderSamePR(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)

	newPR := pullrequest.NewNewPullRequestDetected(pr)
	activity := testutil.NewTestActivity(
		pullrequest.ActivityTypeComment,
		time.Now(),
		testutil.WithActivityPR(pr.URL(), pr.Number()),
		testutil.WithActivityAuthor("alice"),
	)
	activityEv := pullrequest.NewActivityDetected(pr, activity)

	batch := newBatch()
	batch = accumulateEvent(batch, &newPR, "")
	batch = accumulateEvent(batch, &activityEv, "")

	require.Len(t, batch, 1, "both events must land in the same PR bucket")
	notif := batch[pr.URL()]
	assert.True(t, notif.IsNew)
	assert.Len(t, notif.Activities, 1)
}

func TestAccumulateEvent_DifferentPRs_ProduceSeparateBuckets(t *testing.T) {
	pr1 := testutil.NewTestPullRequest(1, testutil.WithURL("https://github.com/owner/repo/pull/1"))
	pr2 := testutil.NewTestPullRequest(2, testutil.WithURL("https://github.com/owner/repo/pull/2"))

	e1 := pullrequest.NewNewPullRequestDetected(pr1)
	e2 := pullrequest.NewNewPullRequestDetected(pr2)

	batch := newBatch()
	batch = accumulateEvent(batch, &e1, "")
	batch = accumulateEvent(batch, &e2, "")

	assert.Len(t, batch, 2)
}

// ── Unknown / ignored event types ─────────────────────────────────────────────

func TestAccumulateEvent_UnknownEvent_LeavesMapUnchanged(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	event := pullrequest.NewStatusChanged(pr, pullrequest.StatusOpen, pullrequest.StatusMerged)

	batch := accumulateEvent(newBatch(), &event, "")

	assert.Empty(t, batch, "StatusChanged is not an accumulation target")
}
