package pullrequest_test

import (
	"testing"
	"time"

	"github.com/oak/github-notifier/domain/pullrequest"
	"github.com/oak/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestActivityCheckScheduler_DeterminePRsToCheck_ThenMarkChecked(t *testing.T) {
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)
	stalePR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(time.Now().Add(-72*time.Hour)))
	prs := []*pullrequest.PullRequest{stalePR}

	result1 := scheduler.DeterminePRsToCheck(prs)
	assert.Len(t, result1.PRsToCheck, 1, "stale PR should be checked first time")

	scheduler.MarkCheckedAt(time.Now(), prs)
	result2 := scheduler.DeterminePRsToCheck(prs)

	assert.Empty(t, result2.PRsToCheck, "stale PR should be skipped after marking")
	assert.Equal(t, 1, result2.SkippedCount)
}

func TestActivityCheckScheduler_SeedLastChecked_AffectsDeterminePRsToCheck(t *testing.T) {
	now := time.Now()
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)
	stalePR := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))

	scheduler.SeedLastChecked(stalePR.URL(), now)
	result := scheduler.DeterminePRsToCheck([]*pullrequest.PullRequest{stalePR})

	assert.Empty(t, result.PRsToCheck, "seeded PR should be treated as recently checked")
	assert.Equal(t, 1, result.SkippedCount)
}

func TestActivityCheckScheduler_MarkCheckedAt_MarksAllProvidedPRs(t *testing.T) {
	scheduler := pullrequest.NewActivityCheckScheduler(48, 15)
	now := time.Now()
	stalePR1 := testutil.NewTestPullRequest(1, testutil.WithCreatedAt(now.Add(-72*time.Hour)))
	stalePR2 := testutil.NewTestPullRequest(2, testutil.WithCreatedAt(now.Add(-96*time.Hour)))
	prs := []*pullrequest.PullRequest{stalePR1, stalePR2}

	scheduler.MarkCheckedAt(now, prs)
	result := scheduler.DeterminePRsToCheck(prs)

	assert.Empty(t, result.PRsToCheck, "all marked stale PRs should be skipped")
	assert.Equal(t, 2, result.SkippedCount)
}
