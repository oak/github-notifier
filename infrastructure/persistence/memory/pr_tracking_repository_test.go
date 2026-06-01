package memory_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/persistence/memory"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── LoadAll ───────────────────────────────────────────────────────────────────

func TestPRTrackingRepository_LoadAll_Empty(t *testing.T) {
	repo := memory.NewPRTrackingRepository()

	snapshots, err := repo.LoadAll()

	require.NoError(t, err)
	assert.NotNil(t, snapshots)
	assert.Empty(t, snapshots)
}

// ── Save → LoadAll ────────────────────────────────────────────────────────────

func TestPRTrackingRepository_SaveAndLoad_RoundTrip(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	s1 := testutil.NewTestPullRequest(1)
	s2 := testutil.NewTestPullRequest(2)

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{s1, s2}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, s1.URL(), loaded[0].URL())
	assert.Equal(t, s2.URL(), loaded[1].URL())
}

func TestPRTrackingRepository_Save_ReplacesExistingState(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{testutil.NewTestPullRequest(1), testutil.NewTestPullRequest(2)}))
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{testutil.NewTestPullRequest(3)}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "https://github.com/owner/repo/pull/3", loaded[0].URL())
}

func TestPRTrackingRepository_Save_EmptySlice_ClearsState(t *testing.T) {
	repo := memory.NewPRTrackingRepository()

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{testutil.NewTestPullRequest(1)}))
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestPRTrackingRepository_Save_PreservesAllFields(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	checkTime := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	submittedAt := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)

	pr := testutil.NewTestPullRequest(7,
		testutil.WithURL("https://github.com/owner/repo/pull/7"),
		testutil.WithRepository("owner/repo"),
		testutil.WithAuthor("bob"),
		testutil.WithTitle("Feature branch"),
		testutil.WithDraft(true),
		testutil.WithCreatedAt(time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)),
	)
	pr.SetHeadCommitSHA("deadbeef")
	pr.SetPipelineStatus(pullrequest.PipelineStatusSuccess)
	pr.SetLastActivityCheck(checkTime)
	pr.SetReviews(map[string]*pullrequest.Review{
		"joe": pullrequest.NewReview(testutil.NewTestAuthor("joe"), pullrequest.ReviewStateApproved, submittedAt),
	})

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	got := loaded[0]
	assert.Equal(t, pr.URL(), got.URL())
	assert.Equal(t, pr.Number(), got.Number())
	assert.Equal(t, pr.Repository().NameWithOwner(), got.Repository().NameWithOwner())
	assert.Equal(t, pr.Author().Login(), got.Author().Login())
	assert.Equal(t, pr.Title(), got.Title())
	assert.Equal(t, pr.IsDraft(), got.IsDraft())
	assert.True(t, pr.CreatedAt().Equal(got.CreatedAt()))
	assert.Equal(t, pr.HeadCommitSHA(), got.HeadCommitSHA())
	assert.Equal(t, pr.PipelineStatus(), got.PipelineStatus())
	assert.True(t, pr.LastActivityCheck().Equal(got.LastActivityCheck()))
	require.Len(t, got.Reviews(), 1)
	assert.Equal(t, pullrequest.ReviewStateApproved, got.Reviews()["joe"].State())
	assert.True(t, submittedAt.Equal(got.Reviews()["joe"].SubmittedAt()))
}

// ── LoadAll returns copy (mutation safety) ────────────────────────────────────

func TestPRTrackingRepository_LoadAll_ReturnsCopy(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{testutil.NewTestPullRequest(1)}))

	loaded, _ := repo.LoadAll()
	loaded[0].SetHeadCommitSHA("mutated")

	// Internal state must be unaffected
	loaded2, _ := repo.LoadAll()
	assert.NotEqual(t, "mutated", loaded2[0].HeadCommitSHA())
}

// ── Clear ─────────────────────────────────────────────────────────────────────

func TestPRTrackingRepository_Clear_RemovesAll(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{testutil.NewTestPullRequest(1), testutil.NewTestPullRequest(2)}))

	err := repo.Clear()

	require.NoError(t, err)
	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestPRTrackingRepository_Clear_OnEmptyRepo_NoError(t *testing.T) {
	repo := memory.NewPRTrackingRepository()

	err := repo.Clear()

	require.NoError(t, err)
}

// ── Interface compliance ──────────────────────────────────────────────────────

func TestPRTrackingRepository_ImplementsInterface(t *testing.T) {
	var _ pullrequest.PRTrackingRepository = memory.NewPRTrackingRepository()
}

// ── Seen state round-trips ────────────────────────────────────────────────────

func TestPRTrackingRepository_Save_Seen_RoundTrip(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	pr := testutil.NewTestPullRequest(1)
	pr.MarkAsSeen()

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.True(t, loaded[0].Seen())
}

func TestPRTrackingRepository_Save_UnseenByDefault(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	pr := testutil.NewTestPullRequest(1)

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.False(t, loaded[0].Seen())
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestPRTrackingRepository_Update_ChangesSeen(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	pr := testutil.NewTestPullRequest(1)
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

	pr.MarkAsSeen()
	require.NoError(t, repo.Update(pr))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.True(t, loaded[0].Seen())
}

func TestPRTrackingRepository_Update_NoopWhenPRNotFound(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	pr1 := testutil.NewTestPullRequest(1)
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr1}))

	pr2 := testutil.NewTestPullRequest(2)
	pr2.MarkAsSeen()
	require.NoError(t, repo.Update(pr2))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1, "Update of unknown PR must not add a new entry")
	assert.Equal(t, pr1.URL(), loaded[0].URL())
}
