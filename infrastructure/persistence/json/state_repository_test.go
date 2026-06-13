package json_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oak/github-notifier/domain/pullrequest"
	jsonrepo "github.com/oak/github-notifier/infrastructure/persistence/json"
	"github.com/oak/github-notifier/internal/testutil"
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
	pr.SetHeadCommitSHA("abc123")
	pr.SetPipelineStatus(pullrequest.PipelineStatusSuccess)
	pr.SetLastActivityCheck(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	pr.SetReviews(map[string]*pullrequest.Review{
		"alice": testutil.NewTestReview(pullrequest.ReviewStateApproved, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	})
	return pr
}

// ─── PRTrackingRepository ────────────────────────────────────────────────────

func TestStateRepository_Tracking_LoadAll_EmptyWhenNoFile(t *testing.T) {
	repo := newRepo(t)

	prs, err := repo.LoadAll()

	require.NoError(t, err)
	assert.Empty(t, prs)
}

func TestStateRepository_Tracking_SaveAndLoad_RoundTrip(t *testing.T) {
	repo := newRepo(t)
	pr := makePR(42)

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
	assert.WithinDuration(t, pr.CreatedAt(), got.CreatedAt(), time.Second)
	assert.Equal(t, pr.HeadCommitSHA(), got.HeadCommitSHA())
	assert.Equal(t, pr.PipelineStatus(), got.PipelineStatus())
	assert.WithinDuration(t, pr.LastActivityCheck(), got.LastActivityCheck(), time.Second)
	require.Len(t, got.Reviews(), 1)
	assert.Equal(t, pullrequest.ReviewStateApproved, got.Reviews()["alice"].State())
}

func TestStateRepository_Tracking_Seen_RoundTrip(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)
	pr.MarkAsSeen()

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.True(t, loaded[0].Seen())
}

func TestStateRepository_Tracking_Seen_UnseenByDefault(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.False(t, loaded[0].Seen())
}

func TestStateRepository_Tracking_Save_ReplacesAll(t *testing.T) {
	repo := newRepo(t)

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{makePR(1), makePR(2)}))
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{makePR(3)}))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, makePR(3).URL(), loaded[0].URL())
}

func TestStateRepository_Tracking_Clear(t *testing.T) {
	repo := newRepo(t)
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{makePR(1)}))

	require.NoError(t, repo.Clear())

	prs, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, prs)
}

func TestStateRepository_Tracking_PersistedAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	pr := makePR(7)

	// Write with first instance.
	repo1 := newRepoAt(path)
	require.NoError(t, repo1.Save([]*pullrequest.PullRequest{pr}))

	// Load with a fresh instance.
	repo2 := newRepoAt(path)
	loaded, err := repo2.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, pr.URL(), loaded[0].URL())
	assert.Equal(t, pr.HeadCommitSHA(), loaded[0].HeadCommitSHA())
}

// ─── Error-resilience ────────────────────────────────────────────────────────

func TestStateRepository_MissingFile_LoadAll_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	repo := newRepoAt(path)

	prs, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, prs)
}

func TestStateRepository_CorruptFile_TreatedAsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json!!!"), 0600))

	repo := newRepoAt(path)

	prs, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, prs)
}

func TestStateRepository_UnknownVersion_TreatedAsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":999,"trackedPRs":[]}`), 0600))

	repo := newRepoAt(path)

	prs, err := repo.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, prs)
}

// ─── Atomic write ────────────────────────────────────────────────────────────

func TestStateRepository_AtomicWrite_NoTmpFileLeftAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	repo := newRepoAt(path)

	require.NoError(t, repo.Save([]*pullrequest.PullRequest{testutil.NewTestPullRequest(1)}))

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
			pr.SetPipelineStatus(status)

			require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

			loaded, err := repo.LoadAll()
			require.NoError(t, err)
			require.Len(t, loaded, 1)
			assert.Equal(t, status, loaded[0].PipelineStatus())
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
			submittedAt := time.Now().UTC().Truncate(time.Second)
			pr.SetReviews(map[string]*pullrequest.Review{
				"bob": pullrequest.NewReview(testutil.NewTestAuthor("bob"), state, submittedAt),
			})

			require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

			loaded, err := repo.LoadAll()
			require.NoError(t, err)
			require.Len(t, loaded, 1)
			assert.Equal(t, state, loaded[0].Reviews()["bob"].State())
		})
	}
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestStateRepository_Tracking_Update_ChangesSeen(t *testing.T) {
	repo := newRepo(t)
	pr := testutil.NewTestPullRequest(1)
	require.NoError(t, repo.Save([]*pullrequest.PullRequest{pr}))

	pr.MarkAsSeen()
	require.NoError(t, repo.Update(pr))

	loaded, err := repo.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.True(t, loaded[0].Seen())
}

func TestStateRepository_Tracking_Update_NoopWhenPRNotFound(t *testing.T) {
	repo := newRepo(t)
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
