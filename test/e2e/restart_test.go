//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tempStateFile returns a path inside a new temp dir.  The file does not
// exist yet; it will be created by the StateRepository on first write.
func tempStateFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "state.json")
}

// TestE2E_Restart_NoDuplicateNotifications verifies that PRs seen in run 1
// do NOT trigger NewPullRequestDetected events in run 2.
func TestE2E_Restart_NoDuplicateNotifications(t *testing.T) {
	stateFile := tempStateFile(t)

	// ── Run 1 ──
	run1 := SetupSuiteFromStateFile(t, stateFile)
	defer run1.Teardown()

	run1.mockGitHub.SetupPRs([]MockPR{
		{Title: "Feature A", Number: 5001, Author: "alice", HeadCommitSHA: "aaa111"},
		{Title: "Feature B", Number: 5002, Author: "bob", HeadCommitSHA: "bbb222"},
	})

	err := run1.orchestrator.ExecuteInitialCheck(run1.ctx)
	require.NoError(t, err)
	run1.FlushNotifications()
	run1.ClearNotifications()

	// Do one regular check so state is persisted via TrackPRs.
	err = run1.orchestrator.ExecuteRegularCheck(run1.ctx)
	require.NoError(t, err)
	run1.FlushNotifications()

	// ── Run 2 (simulated restart) ──
	run2 := SetupSuiteOnMockServer(t, run1.mockGitHub, stateFile)
	defer run2.Teardown()

	// Same PRs visible from GitHub.
	err = run2.orchestrator.ExecuteInitialCheck(run2.ctx)
	require.NoError(t, err)
	run2.FlushNotifications()

	notifs := run2.notifications.GetNotifications()
	for _, n := range notifs {
		assert.NotEqual(t, "New PR needing review", n.Title,
			"run 2 must NOT re-notify for PRs that were already seen in run 1 (got notification: %+v)", n)
	}
}

// TestE2E_Restart_PipelineChangeAfterRestart_Notifies verifies that a pipeline
// status change that happens between run 1 and run 2 fires a notification in
// run 2, using the persisted previous status as the baseline.
func TestE2E_Restart_PipelineChangeAfterRestart_Notifies(t *testing.T) {
	stateFile := tempStateFile(t)

	// ── Run 1: PR with SUCCESS pipeline ──
	run1 := SetupSuiteFromStateFile(t, stateFile)
	defer run1.Teardown()

	run1.mockGitHub.SetupPRs([]MockPR{
		{Title: "Pipeline PR", Number: 5100, Author: "alice", HeadCommitSHA: "ccc333", PipelineStatus: "SUCCESS"},
	})

	err := run1.orchestrator.ExecuteInitialCheck(run1.ctx)
	require.NoError(t, err)
	run1.FlushNotifications()
	run1.ClearNotifications()

	err = run1.orchestrator.ExecuteRegularCheck(run1.ctx)
	require.NoError(t, err)
	run1.FlushNotifications()
	run1.ClearNotifications()

	// ── Between runs: pipeline fails ──
	run1.mockGitHub.SetPipelineStatus(5100, "FAILURE")

	// ── Run 2 ──
	run2 := SetupSuiteOnMockServer(t, run1.mockGitHub, stateFile)
	defer run2.Teardown()

	err = run2.orchestrator.ExecuteInitialCheck(run2.ctx)
	require.NoError(t, err)
	run2.FlushNotifications()

	err = run2.orchestrator.ExecuteRegularCheck(run2.ctx)
	require.NoError(t, err)
	run2.FlushNotifications()

	notifs := run2.notifications.GetNotifications()
	foundPipeline := false
	for _, n := range notifs {
		if n.Title == "PR Pipeline Failed 🔴" {
			foundPipeline = true
			break
		}
	}
	assert.True(t, foundPipeline, "run 2 MUST notify about pipeline failure after restart (notifications: %+v)", notifs)
}

// TestE2E_Restart_ClosedPRAfterRestart_Notifies verifies that a PR that was
// open in run 1 but has been merged by run 2 emits a Merged event.
func TestE2E_Restart_ClosedPRAfterRestart_Notifies(t *testing.T) {
	stateFile := tempStateFile(t)

	// ── Run 1: Two open PRs ──
	run1 := SetupSuiteFromStateFile(t, stateFile)
	defer run1.Teardown()

	run1.mockGitHub.SetupPRs([]MockPR{
		{Title: "Will Stay Open", Number: 5200, Author: "alice", HeadCommitSHA: "ddd444"},
		{Title: "Will Be Merged", Number: 5201, Author: "bob", HeadCommitSHA: "eee555"},
	})

	err := run1.orchestrator.ExecuteInitialCheck(run1.ctx)
	require.NoError(t, err)
	run1.FlushNotifications()
	run1.ClearNotifications()

	err = run1.orchestrator.ExecuteRegularCheck(run1.ctx)
	require.NoError(t, err)
	run1.FlushNotifications()
	run1.ClearNotifications()

	// ── Between runs: PR 5201 is merged on GitHub ──
	// Mark it merged in the mock but do NOT replace the PR list with SetupPRs:
	// the PR must remain in m.prs (with State=="merged") so that FetchPRStatus
	// can return MERGED when DetectClosedPRs queries it.  The open-PR search
	// query naturally excludes non-open PRs, so it won't appear in run 2's fetch.
	run1.mockGitHub.MergePR(5201)

	// ── Run 2 ──
	run2 := SetupSuiteOnMockServer(t, run1.mockGitHub, stateFile)
	defer run2.Teardown()

	err = run2.orchestrator.ExecuteInitialCheck(run2.ctx)
	require.NoError(t, err)
	run2.FlushNotifications()

	err = run2.orchestrator.ExecuteRegularCheck(run2.ctx)
	require.NoError(t, err)
	run2.FlushNotifications()

	notifs := run2.notifications.GetNotifications()
	foundMerge := false
	for _, n := range notifs {
		if n.Title == "PR Merged" {
			foundMerge = true
			break
		}
	}
	assert.True(t, foundMerge, "run 2 MUST notify about merged PR after restart (notifications: %+v)", notifs)
}

// TestE2E_Restart_NewPRAfterRestart_Notifies verifies that a PR that did not
// exist in run 1 fires a NewPullRequestDetected event in run 2.
func TestE2E_Restart_NewPRAfterRestart_Notifies(t *testing.T) {
	stateFile := tempStateFile(t)

	// ── Run 1: One PR ──
	run1 := SetupSuiteFromStateFile(t, stateFile)
	defer run1.Teardown()

	run1.mockGitHub.SetupPRs([]MockPR{
		{Title: "Existing PR", Number: 5300, Author: "alice", HeadCommitSHA: "fff666"},
	})

	err := run1.orchestrator.ExecuteInitialCheck(run1.ctx)
	require.NoError(t, err)
	run1.FlushNotifications()
	run1.ClearNotifications()

	err = run1.orchestrator.ExecuteRegularCheck(run1.ctx)
	require.NoError(t, err)
	run1.FlushNotifications()
	run1.ClearNotifications()

	// ── Between runs: a new PR is opened ──
	run1.mockGitHub.SetupPRs([]MockPR{
		{Title: "Existing PR", Number: 5300, Author: "alice", HeadCommitSHA: "fff666"},
		{Title: "Brand New PR", Number: 5301, Author: "charlie", HeadCommitSHA: "ggg777"},
	})

	// ── Run 2 ──
	run2 := SetupSuiteOnMockServer(t, run1.mockGitHub, stateFile)
	defer run2.Teardown()

	err = run2.orchestrator.ExecuteInitialCheck(run2.ctx)
	require.NoError(t, err)
	run2.FlushNotifications()

	err = run2.orchestrator.ExecuteRegularCheck(run2.ctx)
	require.NoError(t, err)
	run2.FlushNotifications()

	notifs := run2.notifications.GetNotifications()
	foundNew := false
	for _, n := range notifs {
		if n.Title == "New PR needing review" {
			foundNew = true
			break
		}
	}
	assert.True(t, foundNew, "run 2 MUST notify about new PR added between restarts (notifications: %+v)", notifs)

	// The existing PR must NOT generate a new-PR notification.
	for _, n := range notifs {
		if n.Title == "New PR needing review" {
			assert.NotContains(t, n.Body, "Existing PR",
				"must not re-notify for PR that was already seen in run 1")
		}
	}
}

// TestE2E_Restart_MissingStateFile_BehavesAsFirstRun verifies that when the
// state file does not exist, the app behaves identically to a first run:
// existing PRs are silenced and no NewPullRequestDetected events are emitted.
func TestE2E_Restart_MissingStateFile_BehavesAsFirstRun(t *testing.T) {
	// Use a path that definitely does not exist.
	stateFile := filepath.Join(t.TempDir(), "nonexistent", "state.json")

	suite := SetupSuiteFromStateFile(t, stateFile)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "Pre-existing PR", Number: 5400, Author: "alice", HeadCommitSHA: "hhh888"},
	})

	err := suite.orchestrator.ExecuteInitialCheck(suite.ctx)
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	for _, n := range notifs {
		assert.NotEqual(t, "New PR needing review", n.Title,
			"first run with missing state file must silence existing PRs")
	}
}

// TestE2E_Restart_CorruptStateFile_BehavesAsFirstRun verifies that when the
// state file is corrupt, the app recovers gracefully and treats it as a first run.
func TestE2E_Restart_CorruptStateFile_BehavesAsFirstRun(t *testing.T) {
	stateFile := tempStateFile(t)
	require.NoError(t, os.WriteFile(stateFile, []byte("{corrupt json!!!"), 0600))

	suite := SetupSuiteFromStateFile(t, stateFile)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "Some PR", Number: 5500, Author: "alice", HeadCommitSHA: "iii999"},
	})

	err := suite.orchestrator.ExecuteInitialCheck(suite.ctx)
	require.NoError(t, err)
	suite.FlushNotifications()

	// First run with corrupt state → existing PRs are silenced.
	notifs := suite.notifications.GetNotifications()
	for _, n := range notifs {
		assert.NotEqual(t, "New PR needing review", n.Title,
			"corrupt state file should trigger first-run silencing")
	}
}
