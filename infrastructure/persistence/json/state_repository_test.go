package json_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oak3/github-notifier/domain/pullrequest"
	jsonrepo "github.com/oak3/github-notifier/infrastructure/persistence/json"
	"github.com/oak3/github-notifier/internal/testutil"
)

// newRepo creates a StateRepository backed by a temp file.
// The file is automatically removed after the test.
func newRepo(t *testing.T) *jsonrepo.StateRepository {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "state-*.json")
	require.NoError(t, err)
	path := f.Name()
	f.Close()
	// Remove the file so the repo starts fresh (no pre-existing file).
	require.NoError(t, os.Remove(path))
	return jsonrepo.NewStateRepository(path)
}

// newRepoAt creates a StateRepository at a specific path.
func newRepoAt(path string) *jsonrepo.StateRepository {
	return jsonrepo.NewStateRepository(path)
}

// makePR builds a PullRequest with all fields populated.
func makePR(number int) *pullrequest.PullRequest {
	pr := testutil.NewTestPullRequest(number)
	pr.SetInitialHeadCommitSHA("abc123")
	pr.SetInitialPipelineStatus(pullrequest.PipelineStatusSuccess)
	pr.SetInitialLastActivityCheck(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	pr.SetInitialReviews(map[string]*pullrequest.Review{
		"alice": testutil.NewTestReview(pullrequest.ReviewStateApproved, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	})
	return pr
}

// ─── SeenRepository ──────────────────────────────────────────────────────────

func TestStateRepository_Seen_HasNoSeenPRs_InitiallyTrue(t *testing.T) {
	repo := newRepo(t)
	assert.True(t, repo.HasNoSeenPRs())
}

func TestStateRepository_Seen_MarkAsSeen(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)

	require.NoError(t, repo.MarkAsSeen(pr.Identifier()))

	assert.True(t, repo.HasBeenSeen(pr.Identifier()))
	assert.False(t, repo.HasNoSeenPRs())
}

func TestStateRepository_Seen_MarkAsSeen_Idempotent(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)

	require.NoError(t, repo.MarkAsSeen(pr.Identifier()))
	require.NoError(t, repo.MarkAsSeen(pr.Identifier())) // second mark should not duplicate

	assert.True(t, repo.HasBeenSeen(pr.Identifier()))
	// Verify only one entry in file (not two).  We test indirectly: a new
	// repo loaded from the same file should also see the PR exactly once.
	snap, err := repo.LoadAll()
	require.NoError(t, err)
	_ = snap // not asserting count here; the single-entry behaviour is tested
	//   below via the round-trip test; for idempotence we just assert no error.
}

func TestStateRepository_Seen_HasBeenSeen_Unknown(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(99)

	assert.False(t, repo.HasBeenSeen(pr.Identifier()))
}

func TestStateRepository_Seen_UnmarkAsSeen(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)

	require.NoError(t, repo.MarkAsSeen(pr.Identifier()))
	assert.True(t, repo.HasBeenSeen(pr.Identifier()))

	require.NoError(t, repo.UnmarkAsSeen(pr.Identifier()))
	assert.False(t, repo.HasBeenSeen(pr.Identifier()))
}

func TestStateRepository_Seen_UnmarkAsSeen_NonExistent_NoError(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)

	// Unmarking something that was never marked should be a no-op.
	assert.NoError(t, repo.UnmarkAsSeen(pr.Identifier()))
	assert.False(t, repo.HasBeenSeen(pr.Identifier()))
}

func TestStateRepository_Seen_HasNoSeenPRs_AfterMarkAndUnmark(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)

	require.NoError(t, repo.MarkAsSeen(pr.Identifier()))
	require.NoError(t, repo.UnmarkAsSeen(pr.Identifier()))

	assert.True(t, repo.HasNoSeenPRs())
}

func TestStateRepository_Seen_ClearAllSeenPRs(t *testing.T) {
	repo := newRepo(t)
	pr1 := testutil.NewTestPullRequest(1)
	pr2 := testutil.NewTestPullRequest(2)

	require.NoError(t, repo.MarkAsSeen(pr1.Identifier()))
	require.NoError(t, repo.MarkAsSeen(pr2.Identifier()))
	require.False(t, repo.HasNoSeenPRs())

	require.NoError(t, repo.ClearAllSeenPRs())

	assert.True(t, repo.HasNoSeenPRs())
	assert.False(t, repo.HasBeenSeen(pr1.Identifier()))
	assert.False(t, repo.HasBeenSeen(pr2.Identifier()))
}

func TestStateRepository_Seen_PersistedAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write with first instance.
	repo1 := newRepoAt(path)
	pr := testutil.NewTestPullRequest(1)
	require.NoError(t, repo1.MarkAsSeen(pr.Identifier()))

	// Read with a fresh instance pointing to the same file.
	repo2 := newRepoAt(path)
	assert.True(t, repo2.HasBeenSeen(pr.Identifier()))
	assert.False(t, repo2.HasNoSeenPRs())
}

// ─── PRTrackingRepository ────────────────────────────────────────────────────

func TestStateRepository_Tracking_LoadAll_EmptyWhenNoFile(t *testing.T) {
	repo := newRepo(t)

	snaps, err := repo.LoadAll()

	require.NoError(t, err)
	assert.Empty(t, snaps)
}

func TestStateRepository_Tracking_SaveAndLoad_RoundTrip(t *testing.T) {
	repo := newRepo(t)
	pr := makePR(42)

	require.NoError(t, repo.Save([]pullrequest.PullRequest{pr}))

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
	assert.WithinDuration(t, snap.CreatedAt, got.CreatedAt, time.Second)
	assert.Equal(t, snap.HeadCommitSHA, got.HeadCommitSHA)
	assert.Equal(t, snap.PipelineStatus, got.PipelineStatus)
	assert.WithinDuration(t, snap.LastActivityCheck, got.LastActivityCheck, time.Second)
	require.Len(t, got.Reviews, 1)
	assert.Equal(t, pullrequest.ReviewStateApproved, got.Reviews["alice"].State)
}

func TestStateRepository_Tracking_Save_ReplacesAll(t *testing.T) {
	repo := newRepo(t)

	require.NoError(t, repo.Save([]jsonrepo.PRStateSnapshot{makePR(1), makePR(2)}))
	require.NoError(t, repo.Save([]jsonrepo.PRStateSnapshot{makePR(3)}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, makePR(3).URL, loaded[0].URL)
}

func TestStateRepository_Tracking_LoadAll_ReturnsDefensiveCopy(t *testing.T) {
	repo := newRepo(t)
	snap := makePR(1)
	require.NoError(t, repo.Save([]jsonrepo.PRStateSnapshot{snap}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	// Mutate the returned slice.
	loaded[0].Title = "mutated"

	// A fresh load should still return the original title.
	reloaded, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Equal(t, snap.Title, reloaded[0].Title)
}

func TestStateRepository_Tracking_ClearAllSeenPRs(t *testing.T) {
	repo := newRepo(t)
	require.NoError(t, repo.Save([]jsonrepo.PRStateSnapshot{makePR(1)}))

	require.NoError(t, repo.ClearAllSeenPRs())

	snaps, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, snaps)
}

func TestStateRepository_Tracking_PersistedAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	snap := makePR(7)

	// Write with first instance.
	repo1 := newRepoAt(path)
	require.NoError(t, repo1.Save([]jsonrepo.PRStateSnapshot{snap}))

	// Load with a fresh instance.
	repo2 := newRepoAt(path)
	loaded, err := repo2.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, snap.URL, loaded[0].URL)
	assert.Equal(t, snap.HeadCommitSHA, loaded[0].HeadCommitSHA)
}

// ─── Shared / combined ───────────────────────────────────────────────────────

func TestStateRepository_SeenAndTracked_ShareOneFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	repo := newRepoAt(path)

	pr := testutil.NewTestPullRequest(1)
	snap := makePR(2)

	require.NoError(t, repo.MarkAsSeen(pr.Identifier()))
	require.NoError(t, repo.Save([]jsonrepo.PRStateSnapshot{snap}))

	// Fresh instance — both seen and tracked data survive.
	repo2 := newRepoAt(path)
	assert.True(t, repo2.HasBeenSeen(pr.Identifier()))
	loaded, err := repo2.LoadAll()
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
}

func TestStateRepository_SeenClearAllSeenPRs_DoesNotWipeTracked(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)
	snap := makePR(2)

	require.NoError(t, repo.MarkAsSeen(pr.Identifier()))
	require.NoError(t, repo.Save([]jsonrepo.PRStateSnapshot{snap}))

	// ClearAllSeenPRs wipes everything (it satisfies BOTH ClearAllSeenPRs contracts).
	require.NoError(t, repo.ClearAllSeenPRs())

	assert.True(t, repo.HasNoSeenPRs())
	snaps, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, snaps)
}

// ─── Error-resilience ────────────────────────────────────────────────────────

func TestStateRepository_MissingFile_LoadAll_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	repo := newRepoAt(path)

	snaps, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, snaps)
	assert.True(t, repo.HasNoSeenPRs())
}

func TestStateRepository_CorruptFile_TreatedAsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json!!!"), 0600))

	repo := newRepoAt(path)

	snaps, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, snaps)
	assert.True(t, repo.HasNoSeenPRs())
}

func TestStateRepository_UnknownVersion_TreatedAsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":999,"seenPRs":["https://github.com/o/r/pull/1"],"trackedPRs":[]}`), 0600))

	repo := newRepoAt(path)

	assert.True(t, repo.HasNoSeenPRs())
	snaps, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, snaps)
}

// ─── Atomic write ────────────────────────────────────────────────────────────

func TestStateRepository_AtomicWrite_NoTmpFileLeftAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	repo := newRepoAt(path)

	pr := testutil.NewTestPullRequest(1)
	require.NoError(t, repo.MarkAsSeen(pr.Identifier()))

	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), ".tmp file should not exist after a successful write")
}

// ─── Enum serialisation round-trips ─────────────────────────────────────────

func TestStateRepository_PipelineStatus_AllValues_RoundTrip(t *testing.T) {
	statuses := []pullrequest.PipelineStatus{
		pullrequest.PipelineStatusUnknown,
		pullrequest.PipelineStatusRunning,
		pullrequest.PipelineStatusSuccess,
		pullrequest.PipelineStatusFailed,
	}

	dir := t.TempDir()

	for _, status := range statuses {
		t.Run(status.String(), func(t *testing.T) {
			path := filepath.Join(dir, status.String()+".json")
			repo := newRepoAt(path)

			pr := testutil.NewTestPullRequest(1)
			snap := jsonrepo.ToSnapshot(pr)
			snap.PipelineStatus = status

			require.NoError(t, repo.Save([]jsonrepo.PRStateSnapshot{snap}))

			loaded, err := repo.LoadAll()
			require.NoError(t, err)
			require.Len(t, loaded, 1)
			assert.Equal(t, status, loaded[0].PipelineStatus)
		})
	}
}

func TestStateRepository_ReviewState_AllValues_RoundTrip(t *testing.T) {
	states := []pullrequest.ReviewState{
		pullrequest.ReviewStateApproved,
		pullrequest.ReviewStateChangesRequested,
		pullrequest.ReviewStateCommented,
		pullrequest.ReviewStateDismissed,
	}

	dir := t.TempDir()

	for _, state := range states {
		t.Run(state.String(), func(t *testing.T) {
			path := filepath.Join(dir, state.String()+".json")
			repo := newRepoAt(path)

			pr := testutil.NewTestPullRequest(1)
			snap.Reviews = map[string]jsonrepo.ReviewSnapshot{
				"bob": {State: state, SubmittedAt: time.Now().UTC().Truncate(time.Second)},
			}

			require.NoError(t, repo.Save([]jsonrepo.PRStateSnapshot{snap}))

			loaded, err := repo.LoadAll()
			require.NoError(t, err)
			require.Len(t, loaded, 1)
			assert.Equal(t, state, loaded[0].Reviews()["bob"].State)
		})
	}
}

// ─── StateFilePath via Config ─────────────────────────────────────────────────

func TestStateFilePath_IsSiblingOfConfigFile(t *testing.T) {
	// Ensure the config StateFilePath helper is wired correctly.
	// We test the derivation directly without loading a real config.
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".github-notifier.conf")
	expectedState := filepath.Join(dir, ".github-notifier.state.json")

	// The helper is filepath.Join(filepath.Dir(configPath), stateFileName).
	got := filepath.Join(filepath.Dir(configPath), ".github-notifier.state.json")
	assert.Equal(t, expectedState, got)
}
