package json

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"

	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── ToSnapshot ────────────────────────────────────────────────────────────────

func TestToSnapshot_BasicFields(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	pr := testutil.NewTestPullRequest(42,
		testutil.WithURL("https://github.com/owner/repo/pull/42"),
		testutil.WithTitle("My feature"),
		testutil.WithRepository("owner/repo"),
		testutil.WithAuthor("alice"),
		testutil.WithCreatedAt(createdAt),
		testutil.WithDraft(true),
	)

	snap := toSnapshot(pr)

	assert.Equal(t, "https://github.com/owner/repo/pull/42", snap.URL)
	assert.Equal(t, 42, snap.Number)
	assert.Equal(t, "owner/repo", snap.Repository)
	assert.Equal(t, "alice", snap.Author)
	assert.Equal(t, "My feature", snap.Title)
	assert.True(t, snap.IsDraft)
	assert.True(t, snap.CreatedAt.Equal(createdAt))
	assert.Equal(t, pullrequest.PipelineStatusUnknown, snap.PipelineStatus)
	assert.True(t, snap.LastActivityCheck.IsZero())
	assert.Empty(t, snap.HeadCommitSHA)
	assert.Empty(t, snap.Reviews)
}

func TestToSnapshot_WithHeadSHAAndPipelineStatus(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	pr.SetInitialHeadCommitSHA("abc123def456")
	pr.SetInitialPipelineStatus(pullrequest.PipelineStatusSuccess)

	snap := toSnapshot(pr)

	assert.Equal(t, "abc123def456", snap.HeadCommitSHA)
	assert.Equal(t, pullrequest.PipelineStatusSuccess, snap.PipelineStatus)
}

func TestToSnapshot_WithLastActivityCheck(t *testing.T) {
	checkTime := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	pr := testutil.NewTestPullRequest(1)
	pr.SetInitialLastActivityCheck(checkTime)

	snap := toSnapshot(pr)

	assert.True(t, snap.LastActivityCheck.Equal(checkTime))
}

func TestToSnapshot_WithReviews(t *testing.T) {
	pr := testutil.NewTestPullRequest(1)
	submittedAt := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	pr.SetInitialReviews(map[string]*pullrequest.Review{
		"joe":   pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, submittedAt),
		"alice": pullrequest.NewReview(testutil.NewTestAuthor("alice"), pullrequest.ReviewStateChangesRequested, submittedAt),
	})

	snap := toSnapshot(pr)

	require.Len(t, snap.Reviews, 2)
	assert.Equal(t, pullrequest.ReviewStateApproved, snap.Reviews["joe"].State)
	assert.True(t, snap.Reviews["joe"].SubmittedAt.Equal(submittedAt))
	assert.Equal(t, pullrequest.ReviewStateChangesRequested, snap.Reviews["alice"].State)
}

func TestToSnapshot_DoesNotIncludeActivities(t *testing.T) {
	// Activities are never part of the snapshot — they are re-fetched from GitHub.
	pr := testutil.NewTestPullRequest(1)
	pr.AddActivity(testutil.NewTestActivity(pullrequest.ActivityTypeComment, time.Now()))

	snap := toSnapshot(pr)

	// Snapshots have no activity field — just verify the snapshot is well-formed.
	assert.Equal(t, 1, snap.Number)
}

// ── ReconstitutePRFromSnapshot ────────────────────────────────────────────────

func TestReconstitutePRFromSnapshot_RoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	lastCheck := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	submittedAt := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	original := PRStateSnapshot{
		URL:               "https://github.com/owner/repo/pull/42",
		Number:            42,
		Repository:        "owner/repo",
		Author:            "alice",
		Title:             "My feature",
		IsDraft:           true,
		CreatedAt:         createdAt,
		HeadCommitSHA:     "abc123",
		PipelineStatus:    pullrequest.PipelineStatusFailed,
		LastActivityCheck: lastCheck,
		Reviews: map[string]ReviewSnapshot{
			"joe": {State: pullrequest.ReviewStateApproved, SubmittedAt: submittedAt},
		},
	}

	pr, err := original.toDomain()
	require.NoError(t, err)

	restored := toSnapshot(pr)

	assert.Equal(t, original.URL, restored.URL)
	assert.Equal(t, original.Number, restored.Number)
	assert.Equal(t, original.Repository, restored.Repository)
	assert.Equal(t, original.Author, restored.Author)
	assert.Equal(t, original.Title, restored.Title)
	assert.Equal(t, original.IsDraft, restored.IsDraft)
	assert.True(t, original.CreatedAt.Equal(restored.CreatedAt))
	assert.Equal(t, original.HeadCommitSHA, restored.HeadCommitSHA)
	assert.Equal(t, original.PipelineStatus, restored.PipelineStatus)
	assert.True(t, original.LastActivityCheck.Equal(restored.LastActivityCheck))
	require.Len(t, restored.Reviews, 1)
	assert.Equal(t, pullrequest.ReviewStateApproved, restored.Reviews["joe"].State)
	assert.True(t, submittedAt.Equal(restored.Reviews["joe"].SubmittedAt))
}

func TestReconstitutePRFromSnapshot_IsOpen(t *testing.T) {
	// Only open PRs are ever persisted; reconstituted PRs must always be open.
	snap := minimalSnapshot(1)

	pr, err := snap.toDomain()

	require.NoError(t, err)
	assert.True(t, pr.IsOpen())
}

func TestReconstitutePRFromSnapshot_NoEvents(t *testing.T) {
	// Reconstitution restores known state — it must not raise any domain events.
	// Verified by checking Close returns exactly one event (not extras from reconstitution).
	snap := minimalSnapshot(1)
	snap.Reviews = map[string]ReviewSnapshot{
		"joe": {State: pullrequest.ReviewStateApproved, SubmittedAt: time.Now()},
	}

	pr, err := snap.toDomain()
	require.NoError(t, err)

	// Reconstitution uses SetInitial* methods — pure state setters that never raise events.
	// The first command after reconstitution should produce exactly one event, nothing more.
	events := pr.Close()
	assert.Len(t, events, 1, "reconstitution must not pre-populate events")
}

func TestReconstitutePRFromSnapshot_BehavesCorrectly_CloseRaisesEvent(t *testing.T) {
	// After reconstitution the PR should behave like any other PR.
	snap := minimalSnapshot(1)

	pr, err := snap.toDomain()
	require.NoError(t, err)

	events := pr.Close()
	require.Len(t, events, 1)
	_, ok := events[0].(*pullrequest.Closed)
	assert.True(t, ok, "Close should raise Closed event")
}

func TestReconstitutePRFromSnapshot_BehavesCorrectly_SameReviewStateNoEvent(t *testing.T) {
	// Restored reviews should suppress duplicate ReviewStateChanged events.
	submittedAt := time.Now().Add(-1 * time.Hour)
	snap := minimalSnapshot(1)
	snap.Reviews = map[string]ReviewSnapshot{
		"joe": {State: pullrequest.ReviewStateApproved, SubmittedAt: submittedAt},
	}

	pr, err := snap.toDomain()
	require.NoError(t, err)

	// Re-applying the same review state must produce no event.
	events := pr.AddReview(pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, time.Now()))
	assert.Empty(t, events, "same review state after reconstitution must not re-fire event")
}

func TestReconstitutePRFromSnapshot_NilReviews_Succeeds(t *testing.T) {
	snap := minimalSnapshot(1)
	snap.Reviews = nil

	pr, err := snap.toDomain()

	require.NoError(t, err)
	assert.Empty(t, pr.Reviews())
}

func TestReconstitutePRFromSnapshot_InvalidURL_ReturnsError(t *testing.T) {
	snap := minimalSnapshot(1)
	snap.URL = ""

	_, err := snap.toDomain()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reconstitute PR")
}

func TestReconstitutePRFromSnapshot_InvalidNumber_ReturnsError(t *testing.T) {
	snap := minimalSnapshot(0) // number 0 is invalid

	_, err := snap.toDomain()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reconstitute PR")
}

func TestReconstitutePRFromSnapshot_EmptyTitle_ReturnsError(t *testing.T) {
	snap := minimalSnapshot(1)
	snap.Title = ""

	_, err := snap.toDomain()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestReconstitutePRFromSnapshot_ZeroCreatedAt_ReturnsError(t *testing.T) {
	snap := minimalSnapshot(1)
	snap.CreatedAt = time.Time{}

	_, err := snap.toDomain()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "createdAt")
}

func TestReconstitutePRFromSnapshot_InvalidRepository_ReturnsError(t *testing.T) {
	snap := minimalSnapshot(1)
	snap.Repository = "not-valid" // missing slash

	_, err := snap.toDomain()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reconstitute PR")
}

// ── Enum JSON marshal/unmarshal ───────────────────────────────────────────────

func TestPRStatus_JSONMarshalUnmarshal(t *testing.T) {
	cases := []struct {
		status   pullrequest.PRStatus
		wantJSON string
	}{
		{pullrequest.StatusOpen, `"open"`},
		{pullrequest.StatusMerged, `"merged"`},
		{pullrequest.StatusClosed, `"closed"`},
	}
	for _, tc := range cases {
		t.Run(tc.wantJSON, func(t *testing.T) {
			data, err := json.Marshal(tc.status)
			require.NoError(t, err)
			assert.Equal(t, tc.wantJSON, string(data))

			var got pullrequest.PRStatus
			require.NoError(t, json.Unmarshal(data, &got))
			assert.Equal(t, tc.status, got)
		})
	}
}

func TestPRStatus_UnmarshalText_UnknownValue_ReturnsError(t *testing.T) {
	var s pullrequest.PRStatus
	err := json.Unmarshal([]byte(`"something_else"`), &s)
	assert.Error(t, err)
}

func TestReviewState_JSONMarshalUnmarshal(t *testing.T) {
	cases := []struct {
		state    pullrequest.ReviewState
		wantJSON string
	}{
		{pullrequest.ReviewStateApproved, `"approved"`},
		{pullrequest.ReviewStateChangesRequested, `"changes_requested"`},
		{pullrequest.ReviewStateCommented, `"commented"`},
		{pullrequest.ReviewStateDismissed, `"dismissed"`},
	}
	for _, tc := range cases {
		t.Run(tc.wantJSON, func(t *testing.T) {
			data, err := json.Marshal(tc.state)
			require.NoError(t, err)
			assert.Equal(t, tc.wantJSON, string(data))

			var got pullrequest.ReviewState
			require.NoError(t, json.Unmarshal(data, &got))
			assert.Equal(t, tc.state, got)
		})
	}
}

func TestReviewState_UnmarshalText_UnknownValue_ReturnsError(t *testing.T) {
	var rs pullrequest.ReviewState
	err := json.Unmarshal([]byte(`"APPROVED"`), &rs) // GitHub API casing must not be accepted
	assert.Error(t, err)
}

func TestPipelineStatus_JSONMarshalUnmarshal(t *testing.T) {
	cases := []struct {
		status   pullrequest.PipelineStatus
		wantJSON string
	}{
		{pullrequest.PipelineStatusUnknown, `"unknown"`},
		{pullrequest.PipelineStatusRunning, `"running"`},
		{pullrequest.PipelineStatusSuccess, `"success"`},
		{pullrequest.PipelineStatusFailed, `"failed"`},
	}
	for _, tc := range cases {
		t.Run(tc.wantJSON, func(t *testing.T) {
			data, err := json.Marshal(tc.status)
			require.NoError(t, err)
			assert.Equal(t, tc.wantJSON, string(data))

			var got pullrequest.PipelineStatus
			require.NoError(t, json.Unmarshal(data, &got))
			assert.Equal(t, tc.status, got)
		})
	}
}

func TestPipelineStatus_UnmarshalText_UnknownValue_ReturnsError(t *testing.T) {
	var p pullrequest.PipelineStatus
	err := json.Unmarshal([]byte(`"PENDING"`), &p) // GitHub API casing must not be accepted
	assert.Error(t, err)
}

// ── Full JSON round-trip through PRStateSnapshot ─────────────────────────────

func TestPRStateSnapshot_JSONRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	lastCheck := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	submittedAt := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)

	original := PRStateSnapshot{
		URL:               "https://github.com/owner/repo/pull/7",
		Number:            7,
		Repository:        "owner/repo",
		Author:            "bob",
		Title:             "Add tests",
		IsDraft:           false,
		CreatedAt:         createdAt,
		HeadCommitSHA:     "deadbeef",
		PipelineStatus:    pullrequest.PipelineStatusRunning,
		LastActivityCheck: lastCheck,
		Reviews: map[string]ReviewSnapshot{
			"joe":   {State: pullrequest.ReviewStateApproved, SubmittedAt: submittedAt},
			"alice": {State: pullrequest.ReviewStateDismissed, SubmittedAt: submittedAt},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Sanity-check the emitted JSON uses strings for the enums.
	assert.Contains(t, string(data), `"pipelineStatus":"running"`)
	assert.Contains(t, string(data), `"state":"approved"`)

	var restored PRStateSnapshot
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Equal(t, original.URL, restored.URL)
	assert.Equal(t, original.Number, restored.Number)
	assert.Equal(t, original.HeadCommitSHA, restored.HeadCommitSHA)
	assert.Equal(t, original.PipelineStatus, restored.PipelineStatus)
	assert.True(t, original.CreatedAt.Equal(restored.CreatedAt))
	assert.True(t, original.LastActivityCheck.Equal(restored.LastActivityCheck))
	require.Len(t, restored.Reviews, 2)
	assert.Equal(t, pullrequest.ReviewStateApproved, restored.Reviews["joe"].State)
	assert.Equal(t, pullrequest.ReviewStateDismissed, restored.Reviews["alice"].State)
	assert.True(t, submittedAt.Equal(restored.Reviews["joe"].SubmittedAt))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// minimalSnapshot returns the smallest valid PRStateSnapshot for the given PR number.
func minimalSnapshot(number int) PRStateSnapshot {
	return PRStateSnapshot{
		URL:        fmt.Sprintf("https://github.com/owner/repo/pull/%d", number),
		Number:     number,
		Repository: "owner/repo",
		Author:     "alice",
		Title:      "Test PR",
		CreatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Reviews:    map[string]ReviewSnapshot{},
	}
}
