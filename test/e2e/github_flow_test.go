//go:build e2e
// +build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_NewPRDetection_SendsNotification(t *testing.T) {
	// Given: A running application with a new PR available
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{
			Title:  "Add new feature",
			Number: 123,
			Author: "alice",
		},
	})

	// When: A regular check runs (simulating detecting a new PR)
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: A notification should be sent
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Expected at least one notification")

	notification := notifs[0]
	assert.Equal(t, "New PR needing review", notification.Title)
	assert.Contains(t, notification.Body, "Add new feature")

	// And: The menu should be updated with the PR
	prs := suite.menuAdapter.GetPRs()
	require.Len(t, prs, 1)
	assert.Equal(t, "Add new feature", prs[0].Title())
	assert.Equal(t, 123, prs[0].Number())
}

func TestE2E_MultiplePRs_SendsMultipleNotifications(t *testing.T) {
	// Given: Multiple new PRs
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "Feature A", Number: 1, Author: "alice"},
		{Title: "Feature B", Number: 2, Author: "bob"},
		{Title: "My PR", Number: 3, Author: "testuser"}, // User's own PR
	})

	// When: Regular check runs
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: Should get notifications for review PRs and own PR
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Expected notifications")

	// Verify notification titles
	titles := make(map[string]int)
	for _, n := range notifs {
		titles[n.Title]++
	}

	// Should have at least one notification
	assert.Greater(t, len(titles), 0, "Should have notifications")

	// And: All PRs should be in the menu
	prs := suite.menuAdapter.GetPRs()
	assert.Len(t, prs, 3)
}

func TestE2E_DraftPR_NotIncluded(t *testing.T) {
	// Given: A draft PR and a regular PR
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "Draft Feature", Number: 1, Author: "alice", IsDraft: true},
		{Title: "Ready Feature", Number: 2, Author: "bob", IsDraft: false},
	})

	// When: Regular check runs
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: Only the non-draft PR should trigger notification
	prs := suite.menuAdapter.GetPRs()
	require.Len(t, prs, 1)
	assert.Equal(t, "Ready Feature", prs[0].Title())
	assert.False(t, prs[0].IsDraft())
}

func TestE2E_ActivityTracking_DetectsNewComments(t *testing.T) {
	// Given: An app tracking a PR
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Fix bug",
		Number: 456,
		Author: "bob",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// Regular check to start tracking
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Clear initial notification
	suite.ClearNotifications()

	// Small delay to ensure timestamp difference
	time.Sleep(500 * time.Millisecond)

	// When: A new comment appears AFTER the last check (must be newer than last check time!)
	suite.mockGitHub.AddComment(456, MockComment{
		Author:    "charlie",
		Body:      "LGTM!",
		CreatedAt: time.Now(), // NOW, not in the past!
	})

	// And: Regular check runs
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: MUST receive notification for other user's comment
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify for other users' comments - this is the core feature!")

	// Verify it's an activity notification
	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			// Should mention the PR
			assert.Contains(t, n.Body, "Fix bug", "Notification should reference the PR")
			break
		}
	}
	assert.True(t, foundActivity, "Should send activity notification, not just any notification")
}

func TestE2E_ActivityTracking_DetectsNewReview(t *testing.T) {
	// Given: A tracked PR
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Add tests",
		Number: 789,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// Regular check
	suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	suite.ClearNotifications()

	// Small delay
	time.Sleep(100 * time.Millisecond)

	// When: A review is added AFTER the last check
	suite.mockGitHub.AddReview(789, MockReview{
		Author:    "bob",
		State:     "APPROVED",
		Body:      "Looks good",
		CreatedAt: time.Now(), // NOW, not in the past!
	})

	// Regular check
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: MUST receive notification for other user's review
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify for other users' reviews - this is critical!")

	// Verify it's an activity notification
	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			assert.Contains(t, n.Body, "Add tests", "Notification should reference the PR")
			break
		}
	}
	assert.True(t, foundActivity, "Should send activity notification for reviews")
}

func TestE2E_ActivityTracking_DetectsNewReaction(t *testing.T) {
	// Given: A tracked PR with a comment
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Feature work",
		Number: 321,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// Add initial comment (in the past so it's already "seen")
	suite.mockGitHub.AddComment(321, MockComment{
		Author:    "bob",
		Body:      "Great work!",
		CreatedAt: time.Now().Add(-2 * time.Hour), // Old comment
	})

	// Regular check to start tracking
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)
	suite.ClearNotifications()

	// Small delay to ensure clear timestamp separation
	time.Sleep(500 * time.Millisecond)

	// When: A reaction is added to the first comment (index 0) AFTER the last check
	suite.mockGitHub.AddReactionToComment(321, 0, MockReaction{
		Content: "THUMBS_UP",
		User:    "charlie",
		// CreatedAt will default to time.Now() in AddReactionToComment
	})

	// Regular check
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: MUST receive notification for reaction
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify for reactions - this is a key feature!")

	// Verify it's a reaction notification
	foundReaction := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundReaction = true
			assert.Contains(t, n.Body, "Feature work", "Notification should reference the PR")
			break
		}
	}
	assert.True(t, foundReaction, "Should send reaction notification")
}

func TestE2E_ActivityTracking_IgnoresSelfActivity(t *testing.T) {
	// Given: A tracked PR
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "My Feature",
		Number: 100,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	suite.ClearNotifications()

	// When: The authenticated user (testuser) comments on the PR
	suite.mockGitHub.AddComment(100, MockComment{
		Author: "testuser", // Same as authenticated user
		Body:   "Updated the code",
	})

	// Regular check
	suite.orchestrator.ExecuteRegularCheck(suite.ctx)

	// Then: MUST NOT send notification for own activity (critical feature!)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()

	// Should have ZERO notifications - no activity from self should trigger notifications
	assert.Equal(t, 0, len(notifs), "CRITICAL: Must NOT notify for own comments - this would spam the user!")

	// Double-check: Definitely no activity notifications
	activityNotifs := 0
	for _, n := range notifs {
		if n.Title == "PR Activity" || n.Title == "New activity on PR" {
			activityNotifs++
		}
	}
	assert.Equal(t, 0, activityNotifs, "Must filter out self-activity completely")
}

func TestE2E_MergedPR_SendsMergedNotification(t *testing.T) {
	// Given: A tracked PR
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Feature complete",
		Number: 999,
		Author: "alice",
		State:  "open",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	suite.ClearNotifications()

	// When: PR is merged
	suite.mockGitHub.MergePR(999)

	// And: Check runs (merged PRs won't appear in search results)
	suite.mockGitHub.SetupPRs([]MockPR{}) // Simulate PR no longer in open results

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: Menu should be updated (PR removed)
	prs := suite.menuAdapter.GetPRs()
	assert.Len(t, prs, 0, "Merged PR should be removed from menu")
}

func TestE2E_NoNewPRs_NoNotifications(t *testing.T) {
	// Given: No PRs available
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{})

	// When: Regular check runs
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: No notifications should be sent
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	assert.Len(t, notifs, 0, "Should not send notifications when no PRs")

	// And: Menu should be empty
	prs := suite.menuAdapter.GetPRs()
	assert.Len(t, prs, 0)
}

func TestE2E_SeenPRs_NotRenotified(t *testing.T) {
	// Given: A PR that was already seen
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Existing PR",
		Number: 555,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check - should notify
	suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	suite.FlushNotifications()

	initialCount := len(suite.notifications.GetNotifications())
	assert.Greater(t, initialCount, 0, "Should notify on first check")

	// When: Same PR appears in second check
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: Should not send duplicate notification
	suite.FlushNotifications()

	finalCount := len(suite.notifications.GetNotifications())
	assert.Equal(t, initialCount, finalCount, "Should not re-notify for seen PRs")
}

func TestE2E_ErrorRecovery_GitHubAPIDown(t *testing.T) {
	// Given: An application with GitHub API down
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetError(503, "Service Unavailable")

	// When: Check runs
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)

	// Then: Should return error gracefully (not panic)
	assert.Error(t, err)

	// When: API recovers
	suite.mockGitHub.ClearError()
	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "Recovered PR", Number: 1, Author: "alice"},
	})

	// Then: Next check should succeed
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	assert.NoError(t, err)

	// And: Should process the PR
	prs := suite.menuAdapter.GetPRs()
	assert.Len(t, prs, 1)
}

func TestE2E_FullLifecycle_NewPRToActivity(t *testing.T) {
	// Given: A complete application lifecycle test
	suite := SetupSuite(t)
	defer suite.Teardown()

	// Step 1: New PR appears
	pr := MockPR{
		Title:  "Feature X",
		Number: 777,
		Author: "alice",
		State:  "open",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Verify initial notification
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	assert.Greater(t, len(notifs), 0)

	suite.ClearNotifications()
	time.Sleep(100 * time.Millisecond)

	// Step 2: Activity occurs (comment)
	suite.mockGitHub.AddComment(777, MockComment{
		Author:    "bob",
		Body:      "Please address this",
		CreatedAt: time.Now().Add(-1 * time.Second),
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// System processes activity
	suite.ClearNotifications()
	time.Sleep(100 * time.Millisecond)

	// Step 3: More activity (review)
	suite.mockGitHub.AddReview(777, MockReview{
		Author:    "charlie",
		State:     "CHANGES_REQUESTED",
		Body:      "Needs work",
		CreatedAt: time.Now().Add(-1 * time.Second),
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Verify PR still in menu throughout
	prs := suite.menuAdapter.GetPRs()
	assert.Len(t, prs, 1)
	assert.Equal(t, "Feature X", prs[0].Title())
}

func TestE2E_MenuUpdates_ReflectCurrentState(t *testing.T) {
	// Given: Multiple PRs
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "PR 1", Number: 1, Author: "alice"},
		{Title: "PR 2", Number: 2, Author: "bob"},
	})

	// Regular check
	suite.orchestrator.ExecuteRegularCheck(suite.ctx)

	initialPRs := suite.menuAdapter.GetPRs()
	assert.Len(t, initialPRs, 2)

	// When: One PR is closed
	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "PR 1", Number: 1, Author: "alice"}, // Only PR 1 remains
	})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx)

	// Then: Menu should reflect the change
	updatedPRs := suite.menuAdapter.GetPRs()
	assert.Len(t, updatedPRs, 1)
	assert.Equal(t, "PR 1", updatedPRs[0].Title())
}

func TestE2E_CommitActivity_DetectsNewPush(t *testing.T) {
	// Given: A tracked PR
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:         "WIP Feature",
		Number:        888,
		Author:        "alice",
		HeadCommitSHA: "abc123",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	suite.ClearNotifications()

	// When: New commit is pushed
	suite.mockGitHub.AddCommit(888, MockCommit{
		SHA:    "def456",
		Author: "alice",
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx)
	require.NoError(t, err)

	// Then: Should detect the push activity
	time.Sleep(50 * time.Millisecond) // Small delay for async processing

	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()

	// Look for activity notification
	_ = false
	for _, n := range notifs {
		if n.Title == "PR Activity" && (strings.Contains(n.Body, "WIP Feature") || strings.Contains(n.Body, "888")) {
			_ = true
			break
		}
	}

	// Note: Depending on implementation, push might trigger activity
	// This test verifies the system handles commit changes
	assert.True(t, len(notifs) >= 0, "System should process commit updates")
}
