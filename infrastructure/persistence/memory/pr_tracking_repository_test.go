package memory_test

import (
	"testing"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/persistence/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func makeSnapshot(number int) pullrequest.PRStateSnapshot {
	return pullrequest.PRStateSnapshot{
		URL:        "https://github.com/owner/repo/pull/" + itoa(number),
		Number:     number,
		Repository: "owner/repo",
		Author:     "alice",
		Title:      "Test PR",
		CreatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Reviews:    map[string]pullrequest.ReviewSnapshot{},
	}
}

func itoa(n int) string {
	return string(rune('0' + n)) // sufficient for single-digit test numbers
}

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
	s1 := makeSnapshot(1)
	s2 := makeSnapshot(2)

	err := repo.Save([]pullrequest.PRStateSnapshot{s1, s2})
	require.NoError(t, err)

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, s1.URL, loaded[0].URL)
	assert.Equal(t, s2.URL, loaded[1].URL)
}

func TestPRTrackingRepository_Save_ReplacesExistingState(t *testing.T) {
	repo := memory.NewPRTrackingRepository()

	// First save: two snapshots
	require.NoError(t, repo.Save([]pullrequest.PRStateSnapshot{makeSnapshot(1), makeSnapshot(2)}))

	// Second save: only one snapshot — must fully replace the first set
	require.NoError(t, repo.Save([]pullrequest.PRStateSnapshot{makeSnapshot(3)}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "https://github.com/owner/repo/pull/3", loaded[0].URL)
}

func TestPRTrackingRepository_Save_EmptySlice_ClearsState(t *testing.T) {
	repo := memory.NewPRTrackingRepository()

	require.NoError(t, repo.Save([]pullrequest.PRStateSnapshot{makeSnapshot(1)}))
	require.NoError(t, repo.Save([]pullrequest.PRStateSnapshot{}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestPRTrackingRepository_Save_PreservesAllFields(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	checkTime := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	submittedAt := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)

	snap := pullrequest.PRStateSnapshot{
		URL:               "https://github.com/owner/repo/pull/7",
		Number:            7,
		Repository:        "owner/repo",
		Author:            "bob",
		Title:             "Feature branch",
		IsDraft:           true,
		CreatedAt:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		HeadCommitSHA:     "deadbeef",
		PipelineStatus:    pullrequest.PipelineStatusSuccess,
		LastActivityCheck: checkTime,
		Reviews: map[string]pullrequest.ReviewSnapshot{
			"joe": {State: pullrequest.ReviewStateApproved, SubmittedAt: submittedAt},
		},
	}

	require.NoError(t, repo.Save([]pullrequest.PRStateSnapshot{snap}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	got := loaded[0]
	assert.Equal(t, snap.URL, got.URL)
	assert.Equal(t, snap.Number, got.Number)
	assert.Equal(t, snap.Repository, got.Repository)
	assert.Equal(t, snap.Author, got.Author)
	assert.Equal(t, snap.Title, got.Title)
	assert.Equal(t, snap.IsDraft, got.IsDraft)
	assert.True(t, snap.CreatedAt.Equal(got.CreatedAt))
	assert.Equal(t, snap.HeadCommitSHA, got.HeadCommitSHA)
	assert.Equal(t, snap.PipelineStatus, got.PipelineStatus)
	assert.True(t, snap.LastActivityCheck.Equal(got.LastActivityCheck))
	require.Len(t, got.Reviews, 1)
	assert.Equal(t, pullrequest.ReviewStateApproved, got.Reviews["joe"].State)
	assert.True(t, submittedAt.Equal(got.Reviews["joe"].SubmittedAt))
}

// ── LoadAll returns copy (mutation safety) ────────────────────────────────────

func TestPRTrackingRepository_LoadAll_ReturnsCopy(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	require.NoError(t, repo.Save([]pullrequest.PRStateSnapshot{makeSnapshot(1)}))

	loaded, _ := repo.LoadAll()
	loaded[0].Title = "mutated" // mutate the returned slice

	// Internal state must be unaffected
	loaded2, _ := repo.LoadAll()
	assert.Equal(t, "Test PR", loaded2[0].Title)
}

// ── Clear ─────────────────────────────────────────────────────────────────────

func TestPRTrackingRepository_Clear_RemovesAll(t *testing.T) {
	repo := memory.NewPRTrackingRepository()
	require.NoError(t, repo.Save([]pullrequest.PRStateSnapshot{makeSnapshot(1), makeSnapshot(2)}))

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
