//go:build e2e
// +build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/infrastructure/ui"
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
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
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
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
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
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
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
	lastCheck := time.Now()
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
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
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
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
	lastCheck := time.Now()
	suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
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
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
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
	lastCheck := time.Now()
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
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
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
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

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()

	// When: The authenticated user (testuser) comments on the PR
	suite.mockGitHub.AddComment(100, MockComment{
		Author: "testuser", // Same as authenticated user
		Body:   "Updated the code",
	})

	// Regular check
	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())

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

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()

	// When: PR is merged (state changes; search query filters out non-open PRs)
	suite.mockGitHub.MergePR(999)

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)

	// Then: Should receive a merged notification
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Expected merged notification")

	foundMerged := false
	for _, n := range notifs {
		if n.Title == "PR Merged" {
			foundMerged = true
			assert.Contains(t, n.Body, "Feature complete")
			break
		}
	}
	assert.True(t, foundMerged, "Should send 'PR Merged' notification")

	// And: Menu should be updated (PR removed)
	prs := suite.menuAdapter.GetPRs()
	assert.Len(t, prs, 0, "Merged PR should be removed from menu")
}

func TestE2E_ClosedPR_SendsClosedNotification(t *testing.T) {
	// Given: A tracked PR
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Abandoned feature",
		Number: 888,
		Author: "alice",
		State:  "open",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()

	// When: PR is closed without merging
	suite.mockGitHub.ClosePR(888)

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)

	// Then: Should receive a closed notification
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Expected closed notification")

	foundClosed := false
	for _, n := range notifs {
		if n.Title == "PR Closed" {
			foundClosed = true
			assert.Contains(t, n.Body, "Abandoned feature")
			break
		}
	}
	assert.True(t, foundClosed, "Should send 'PR Closed' notification")

	// And: Menu should be updated (PR removed)
	prs := suite.menuAdapter.GetPRs()
	assert.Len(t, prs, 0, "Closed PR should be removed from menu")
}

func TestE2E_MergedPR_NotRedetectedOnNextCheck(t *testing.T) {
	// Given: A PR that was merged and notified
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Already merged",
		Number: 777,
		Author: "alice",
		State:  "open",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()

	// Merge the PR
	suite.mockGitHub.MergePR(777)

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	mergedNotifs := suite.notifications.GetNotifications()
	mergedCount := 0
	for _, n := range mergedNotifs {
		if n.Title == "PR Merged" {
			mergedCount++
		}
	}
	require.Equal(t, 1, mergedCount, "Should get exactly one merged notification")

	// When: Another check runs (PR is still merged, not in open list)
	suite.ClearNotifications()

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should NOT get a duplicate merged notification
	notifs := suite.notifications.GetNotifications()
	for _, n := range notifs {
		assert.NotEqual(t, "PR Merged", n.Title, "Should not re-notify for already-merged PR")
	}
}

func TestE2E_NoNewPRs_NoNotifications(t *testing.T) {
	// Given: No PRs available
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{})

	// When: Regular check runs
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
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
	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.FlushNotifications()

	initialCount := len(suite.notifications.GetNotifications())
	assert.Greater(t, initialCount, 0, "Should notify on first check")

	// When: Same PR appears in second check
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
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
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())

	// Then: Should return error gracefully (not panic)
	assert.Error(t, err)

	// When: API recovers
	suite.mockGitHub.ClearError()
	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "Recovered PR", Number: 1, Author: "alice"},
	})

	// Then: Next check should succeed
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
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

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
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

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
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

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)

	// Verify PR still in menu throughout
	prs := suite.menuAdapter.GetPRs()
	assert.Len(t, prs, 1)
	assert.Equal(t, "Feature X", prs[0].Title())
}

func TestE2E_ActivityOnSeenPR_StillNotifies(t *testing.T) {
	// Regression test: after a PR is seen, subsequent activity must still
	// produce "PR Activity" notifications (not "New PR" and not silence).
	// This tests the interaction between knownPRs, seen repo, and
	// MarkPullRequestAsUnseen across 4 orchestrator cycles.
	suite := SetupSuite(t)
	defer suite.Teardown()

	// Cycle 1: PR detected as new
	pr := MockPR{
		Title:  "Seen PR Activity Test",
		Number: 600,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	lastCheck := time.Now()
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Cycle 1: should get new PR notification")
	assert.Equal(t, "New PR needing review", notifs[0].Title, "Cycle 1: should be 'New PR needing review'")
	suite.ClearNotifications()
	time.Sleep(500 * time.Millisecond)

	// Cycle 2: Comment added → should get "PR Activity"
	suite.mockGitHub.AddComment(600, MockComment{
		Author:    "bob",
		Body:      "First comment",
		CreatedAt: time.Now(),
	})

	now := time.Now()
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	lastCheck = now
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs = suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Cycle 2: MUST notify for activity on seen PR")
	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			break
		}
	}
	assert.True(t, foundActivity, "Cycle 2: should be 'PR Activity', not 'New PR needing review'")
	suite.ClearNotifications()
	time.Sleep(500 * time.Millisecond)

	// Cycle 3: No new activity → should get NO notification
	now = time.Now()
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	lastCheck = now
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs = suite.notifications.GetNotifications()
	assert.Len(t, notifs, 0, "Cycle 3: should NOT re-notify without new activity")
	suite.ClearNotifications()
	time.Sleep(500 * time.Millisecond)

	// Cycle 4: Another comment → should AGAIN get "PR Activity" (not "New PR")
	suite.mockGitHub.AddComment(600, MockComment{
		Author:    "charlie",
		Body:      "Second comment",
		CreatedAt: time.Now(),
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs = suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Cycle 4: MUST notify for second round of activity on seen PR")
	foundActivity = false
	foundNewPR := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
		}
		if n.Title == "New PR needing review" {
			foundNewPR = true
		}
	}
	assert.True(t, foundActivity, "Cycle 4: should be 'PR Activity'")
	assert.False(t, foundNewPR, "Cycle 4: must NOT re-detect as new PR after MarkPullRequestAsUnseen")
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
	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())

	initialPRs := suite.menuAdapter.GetPRs()
	assert.Len(t, initialPRs, 2)

	// When: One PR is closed
	suite.mockGitHub.SetupPRs([]MockPR{
		{Title: "PR 1", Number: 1, Author: "alice"}, // Only PR 1 remains
	})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())

	// Then: Menu should reflect the change
	updatedPRs := suite.menuAdapter.GetPRs()
	assert.Len(t, updatedPRs, 1)
	assert.Equal(t, "PR 1", updatedPRs[0].Title())
}

func TestE2E_CommitActivity_DetectsNewPush(t *testing.T) {
	// Manual test 11: When someone creates a new commit on a PR that I am a reviewer of
	// Given: A tracked PR where I am a reviewer
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:         "WIP Feature",
		Number:        888,
		Author:        "alice",
		HeadCommitSHA: "abc123",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()

	// When: New commit is pushed (head SHA changes)
	suite.mockGitHub.AddCommit(888, MockCommit{
		SHA:    "def456",
		Author: "alice",
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)

	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when new commit is pushed on a PR I'm reviewing")

	// Verify it's an activity notification
	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			assert.Contains(t, n.Body, "WIP Feature", "Notification should reference the PR")
			break
		}
	}
	assert.True(t, foundActivity, "Should send activity notification for push")
}

func TestE2E_OwnerSelfComment_NoNotification(t *testing.T) {
	// Manual test 2: When I comment on my own PR - no notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "My own PR",
		Number: 200,
		Author: "testuser", // PR created by authenticated user
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()
	time.Sleep(100 * time.Millisecond)

	// When: Owner comments on their own PR
	suite.mockGitHub.AddComment(200, MockComment{
		Author:    "testuser",
		Body:      "I'm updating this",
		CreatedAt: time.Now(),
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	assert.Len(t, notifs, 0, "MUST NOT notify when owner comments on their own PR")
}

func TestE2E_ReviewerCommentOnMyPR_Notifies(t *testing.T) {
	// Manual test 3: When a reviewer comments on my PR - notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "My PR for review",
		Number: 300,
		Author: "testuser", // My PR
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	lastCheck := time.Now()
	suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	suite.ClearNotifications()
	time.Sleep(100 * time.Millisecond)

	// When: A reviewer comments on my PR
	suite.mockGitHub.AddComment(300, MockComment{
		Author:    "reviewer1",
		Body:      "Please fix this",
		CreatedAt: time.Now(),
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when reviewer comments on my PR")

	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			assert.Contains(t, n.Body, "My PR for review")
			break
		}
	}
	assert.True(t, foundActivity, "Should be activity notification")
}

func TestE2E_ReviewerReactsToOwnerComment_Notifies(t *testing.T) {
	// Manual test 4: When reviewer reacts to owner comment on PR - notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "My PR with comments",
		Number: 400,
		Author: "testuser", // My PR
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// Add owner's comment (old, already seen)
	suite.mockGitHub.AddComment(400, MockComment{
		Author:    "testuser",
		Body:      "Here's my implementation",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	})

	lastCheck := time.Now()
	suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	suite.ClearNotifications()
	time.Sleep(500 * time.Millisecond)

	// When: Reviewer reacts to owner's comment
	suite.mockGitHub.AddReactionToComment(400, 0, MockReaction{
		Content: "THUMBS_UP",
		User:    "reviewer1",
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when reviewer reacts to owner's comment")

	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			break
		}
	}
	assert.True(t, foundActivity, "Should be activity notification for reaction")
}

func TestE2E_OwnerReactsToReviewerComment_NoNotification(t *testing.T) {
	// Manual test 5 (revisited): When owner reacts to reviewer comment on their own PR
	// This should NOT trigger a notification - it's the owner's own action
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "My PR discussion",
		Number: 500,
		Author: "testuser", // My PR
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// Add reviewer's comment (old, already seen)
	suite.mockGitHub.AddComment(500, MockComment{
		Author:    "reviewer1",
		Body:      "Nice approach!",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()
	time.Sleep(500 * time.Millisecond)

	// When: Owner (authenticated user) reacts to reviewer's comment
	suite.mockGitHub.AddReactionToComment(500, 0, MockReaction{
		Content: "HEART",
		User:    "testuser", // Authenticated user reacts
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	assert.Len(t, notifs, 0, "MUST NOT notify when owner reacts to a comment on their own PR - it's their own action")
}

func TestE2E_OwnerPushesOwnPR_NoNotification(t *testing.T) {
	// Manual test 6: When owner adds a new commit on their own PR - no notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:         "My WIP PR",
		Number:        601,
		Author:        "testuser", // My PR
		HeadCommitSHA: "initial123",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()

	// When: Owner pushes a new commit
	suite.mockGitHub.AddCommit(601, MockCommit{
		SHA:    "newcommit456",
		Author: "testuser",
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	assert.Len(t, notifs, 0, "MUST NOT notify when owner pushes to their own PR")
}

func TestE2E_SomeoneApprovesMyPR_Notifies(t *testing.T) {
	// Manual test 7: When someone approves my PR - notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Ready for review",
		Number: 700,
		Author: "testuser", // My PR
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	lastCheck := time.Now()
	suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	suite.ClearNotifications()
	time.Sleep(100 * time.Millisecond)

	// When: Someone approves my PR
	suite.mockGitHub.AddReview(700, MockReview{
		Author:    "approver",
		State:     "APPROVED",
		Body:      "",
		CreatedAt: time.Now(),
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when someone approves my PR")

	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			assert.Contains(t, n.Body, "Ready for review")
			break
		}
	}
	assert.True(t, foundActivity, "Should be activity notification for approval")
}

func TestE2E_SomeoneRequestsMyReview_Notifies(t *testing.T) {
	// Manual test 8: When someone creates a PR and requests my review - notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{
			Title:  "Please review this",
			Number: 800,
			Author: "colleague", // Someone else's PR
		},
	})

	// When: Regular check detects the PR
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when someone requests my review")
	assert.Equal(t, "New PR needing review", notifs[0].Title)
	assert.Contains(t, notifs[0].Body, "Please review this")
}

func TestE2E_ICommentOnSomeoneElsePR_NoNotification(t *testing.T) {
	// Manual test 9: When I comment on someone else's PR - no notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Someone else feature",
		Number: 900,
		Author: "colleague", // Someone else's PR
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()
	time.Sleep(100 * time.Millisecond)

	// When: I comment on someone else's PR
	suite.mockGitHub.AddComment(900, MockComment{
		Author:    "testuser", // Authenticated user
		Body:      "My review comment",
		CreatedAt: time.Now(),
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	assert.Len(t, notifs, 0, "MUST NOT notify when I comment on someone else's PR")
}

func TestE2E_SomeoneReactsToMyCommentOnOthersPR_Notifies(t *testing.T) {
	// Manual test 10: When someone reacts to a comment I made on someone else's PR - notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Collaborative PR",
		Number: 1000,
		Author: "colleague", // Someone else's PR
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// Add my comment (old, already seen)
	suite.mockGitHub.AddComment(1000, MockComment{
		Author:    "testuser",
		Body:      "I think this needs refactoring",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	})

	lastCheck := time.Now()
	suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	suite.ClearNotifications()
	time.Sleep(500 * time.Millisecond)

	// When: Someone reacts to my comment
	suite.mockGitHub.AddReactionToComment(1000, 0, MockReaction{
		Content: "THUMBS_UP",
		User:    "colleague",
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, lastCheck)
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when someone reacts to my comment on another PR")

	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			break
		}
	}
	assert.True(t, foundActivity, "Should be activity notification for reaction")
}

func TestE2E_NewCommitOnPRImReviewing_Notifies(t *testing.T) {
	// Manual test 11: When someone creates a new commit on a PR that I am a reviewer of
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:         "Feature under review",
		Number:        1100,
		Author:        "colleague",
		HeadCommitSHA: "initial_sha",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check - detect and track PR
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: PR author pushes a new commit
	suite.mockGitHub.AddCommit(1100, MockCommit{
		SHA:    "new_sha_456",
		Author: "colleague",
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when new commit is pushed on a PR I'm reviewing")

	foundActivity := false
	for _, n := range notifs {
		if n.Title == "PR Activity" {
			foundActivity = true
			assert.Contains(t, n.Body, "Feature under review")
			break
		}
	}
	assert.True(t, foundActivity, "Should be activity notification for new push")
}

func TestE2E_IApproveAPR_NoNotification(t *testing.T) {
	// Manual test 12: When I approve a PR - no notification
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Needs my approval",
		Number: 1200,
		Author: "colleague",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	suite.ClearNotifications()
	time.Sleep(100 * time.Millisecond)

	// When: I approve the PR
	suite.mockGitHub.AddReview(1200, MockReview{
		Author:    "testuser", // Authenticated user
		State:     "APPROVED",
		Body:      "LGTM",
		CreatedAt: time.Now(),
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	assert.Len(t, notifs, 0, "MUST NOT notify when I approve a PR - it's my own action")
}

// === Review State Detection E2E Tests (OAK-9) ===

func TestE2E_ReviewStateApproval_SendsReviewNotification(t *testing.T) {
	// Given: A tracked PR with no reviews initially
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Feature for review",
		Number: 1300,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect the PR (no reviews yet)
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: A reviewer approves the PR (latestReviews updated in search response)
	suite.mockGitHub.SetLatestReviews(1300, []MockLatestReview{
		{Author: "bob", State: "APPROVED", SubmittedAt: time.Now()},
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should receive a review state notification
	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when a PR is approved")

	foundReview := false
	for _, n := range notifs {
		if n.Title == "PR Review" {
			foundReview = true
			assert.Contains(t, n.Body, "Feature for review")
			assert.Contains(t, n.Body, "bob")
			assert.Contains(t, n.Body, "approved")
			break
		}
	}
	assert.True(t, foundReview, "Should send PR Review notification for approval")
}

func TestE2E_ReviewStateChangesRequested_SendsReviewNotification(t *testing.T) {
	// Given: A tracked PR with no reviews initially
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Needs rework",
		Number: 1400,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect the PR
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: A reviewer requests changes
	suite.mockGitHub.SetLatestReviews(1400, []MockLatestReview{
		{Author: "charlie", State: "CHANGES_REQUESTED", SubmittedAt: time.Now()},
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should receive a review notification with changes_requested
	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when changes are requested")

	foundReview := false
	for _, n := range notifs {
		if n.Title == "PR Review" {
			foundReview = true
			assert.Contains(t, n.Body, "Needs rework")
			assert.Contains(t, n.Body, "charlie")
			assert.Contains(t, n.Body, "changes_requested")
			break
		}
	}
	assert.True(t, foundReview, "Should send PR Review notification for changes requested")
}

func TestE2E_ReviewStateChange_ChangesRequestedToApproved(t *testing.T) {
	// Given: A tracked PR that already has a changes_requested review
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Iterative review",
		Number: 1500,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect the PR
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// Cycle 2: reviewer requests changes
	suite.mockGitHub.SetLatestReviews(1500, []MockLatestReview{
		{Author: "bob", State: "CHANGES_REQUESTED", SubmittedAt: time.Now()},
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Should notify on initial changes_requested")
	suite.ClearNotifications()

	// Cycle 3: same reviewer now approves (state changed)
	suite.mockGitHub.SetLatestReviews(1500, []MockLatestReview{
		{Author: "bob", State: "APPROVED", SubmittedAt: time.Now()},
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should receive a new review notification for the state change
	notifs = suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when reviewer changes from changes_requested to approved")

	foundApproval := false
	for _, n := range notifs {
		if n.Title == "PR Review" {
			foundApproval = true
			assert.Contains(t, n.Body, "bob")
			assert.Contains(t, n.Body, "approved")
			break
		}
	}
	assert.True(t, foundApproval, "Should send PR Review notification when reviewer changes to approved")
}

func TestE2E_SelfReview_NoReviewNotification(t *testing.T) {
	// Given: A tracked PR where I am a reviewer
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Someone else's PR",
		Number: 1600,
		Author: "colleague",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: I (testuser) submit a review
	suite.mockGitHub.SetLatestReviews(1600, []MockLatestReview{
		{Author: "testuser", State: "APPROVED", SubmittedAt: time.Now()},
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should NOT receive a notification for my own review
	notifs := suite.notifications.GetNotifications()
	reviewNotifs := 0
	for _, n := range notifs {
		if n.Title == "PR Review" {
			reviewNotifs++
		}
	}
	assert.Equal(t, 0, reviewNotifs, "MUST NOT notify for own review - it's my own action")
}

func TestE2E_ReviewStateSameState_NoNotification(t *testing.T) {
	// Given: A tracked PR with an existing approved review
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Already approved",
		Number: 1700,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check with review already present
	suite.mockGitHub.SetLatestReviews(1700, []MockLatestReview{
		{Author: "bob", State: "APPROVED", SubmittedAt: time.Now()},
	})

	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: Same review state appears again (no change)
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should NOT notify (state hasn't changed)
	notifs := suite.notifications.GetNotifications()
	reviewNotifs := 0
	for _, n := range notifs {
		if n.Title == "PR Review" {
			reviewNotifs++
		}
	}
	assert.Equal(t, 0, reviewNotifs, "Should NOT notify when review state hasn't changed")
}

func TestE2E_MultipleReviewers_DetectsAllStateChanges(t *testing.T) {
	// Given: A tracked PR
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "Multi-reviewer PR",
		Number: 1800,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect PR
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: Multiple reviewers submit reviews at once
	suite.mockGitHub.SetLatestReviews(1800, []MockLatestReview{
		{Author: "bob", State: "APPROVED", SubmittedAt: time.Now()},
		{Author: "charlie", State: "CHANGES_REQUESTED", SubmittedAt: time.Now()},
	})

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should receive notification(s) for all review changes
	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify for review state changes from multiple reviewers")

	// Find PR Review notification - both reviewers should be mentioned
	foundReview := false
	for _, n := range notifs {
		if n.Title == "PR Review" {
			foundReview = true
			assert.Contains(t, n.Body, "bob")
			assert.Contains(t, n.Body, "charlie")
			break
		}
	}
	assert.True(t, foundReview, "Should aggregate multiple review changes into a single notification")
}

func TestE2E_MenuDisplaysReviewStates(t *testing.T) {
	// Given: A PR with reviews
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:  "PR with reviews",
		Number: 1900,
		Author: "alice",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// Set up reviews
	suite.mockGitHub.SetLatestReviews(1900, []MockLatestReview{
		{Author: "bob", State: "APPROVED", SubmittedAt: time.Now()},
		{Author: "charlie", State: "CHANGES_REQUESTED", SubmittedAt: time.Now()},
	})

	// When: Check runs and display is updated
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)

	// Then: PRs in the menu should have review data available
	prs := suite.menuAdapter.GetPRs()
	require.Len(t, prs, 1)

	// Verify the PR has review state data
	reviewSummary := prs[0].ReviewSummary()
	assert.False(t, reviewSummary.IsEmpty(), "PR should have review summary")

	// Verify menu format includes review state info
	formatted := ui.FormatReviewSummaryForMenu(reviewSummary)
	assert.NotEmpty(t, formatted, "Review summary should produce formatted output")
	assert.Contains(t, formatted, "✅", "Should show approval emoji")
	assert.Contains(t, formatted, "❌", "Should show changes requested emoji")
}

// === Pipeline Status E2E Tests (OAK-19) ===

func TestE2E_PipelineInitialStatus_ShownInMenuImmediately(t *testing.T) {
	// Given: A PR that already has a known pipeline status when first fetched
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:          "Feature already green",
		Number:         1950,
		Author:         "alice",
		HeadCommitSHA:  "abc000",
		PipelineStatus: "SUCCESS",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// When: first check runs
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)

	// Then: menu should immediately show the pipeline emoji, no second check needed
	prs := suite.menuAdapter.GetPRs()
	require.Len(t, prs, 1)
	assert.Equal(t, pullrequest.PipelineStatusSuccess, prs[0].PipelineStatus(),
		"Pipeline status should be visible in menu after first check")
}

func TestE2E_PipelineSuccess_SendsNotificationAndUpdatesMenu(t *testing.T) {
	// Given: A tracked PR that starts with no pipeline status
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:         "Feature with CI",
		Number:        2000,
		Author:        "alice",
		HeadCommitSHA: "abc123",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect the PR
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: CI completes successfully
	suite.mockGitHub.SetPipelineStatus(2000, "SUCCESS")

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should notify about pipeline success
	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when pipeline succeeds")

	foundPipeline := false
	for _, n := range notifs {
		if n.Title == "PR Pipeline Passed 🟢" {
			foundPipeline = true
			assert.Contains(t, n.Body, "Feature with CI")
			break
		}
	}
	assert.True(t, foundPipeline, "Should send pipeline success notification")

	// And: The PR in the menu should have the correct pipeline status
	prs := suite.menuAdapter.GetPRs()
	require.Len(t, prs, 1)
	assert.Equal(t, pullrequest.PipelineStatusSuccess, prs[0].PipelineStatus())
}

func TestE2E_PipelineFailure_SendsNotificationAndUpdatesMenu(t *testing.T) {
	// Given: A tracked PR with a running pipeline
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:          "Broken feature",
		Number:         2100,
		Author:         "alice",
		HeadCommitSHA:  "def456",
		PipelineStatus: "IN_PROGRESS",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect the PR with running pipeline
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: CI fails
	suite.mockGitHub.SetPipelineStatus(2100, "FAILURE")

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should notify about pipeline failure
	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when pipeline fails")

	foundPipeline := false
	for _, n := range notifs {
		if n.Title == "PR Pipeline Failed 🔴" {
			foundPipeline = true
			assert.Contains(t, n.Body, "Broken feature")
			break
		}
	}
	assert.True(t, foundPipeline, "Should send pipeline failure notification")

	// And: The PR in the menu should reflect the failure
	prs := suite.menuAdapter.GetPRs()
	require.Len(t, prs, 1)
	assert.Equal(t, pullrequest.PipelineStatusFailed, prs[0].PipelineStatus())
}

func TestE2E_PipelineRunning_SendsNotification(t *testing.T) {
	// Given: A tracked PR with no pipeline status yet
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:         "Work in progress",
		Number:        2200,
		Author:        "alice",
		HeadCommitSHA: "ghi789",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect the PR
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: CI starts running
	suite.mockGitHub.SetPipelineStatus(2200, "IN_PROGRESS")

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should notify that pipeline is running
	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when pipeline starts running")

	foundPipeline := false
	for _, n := range notifs {
		if n.Title == "PR Pipeline Running 🟡" {
			foundPipeline = true
			break
		}
	}
	assert.True(t, foundPipeline, "Should send pipeline running notification")

	// And: Menu should show running status
	prs := suite.menuAdapter.GetPRs()
	require.Len(t, prs, 1)
	assert.Equal(t, pullrequest.PipelineStatusRunning, prs[0].PipelineStatus())
}

func TestE2E_PipelineSameStatus_NoNotification(t *testing.T) {
	// Given: A tracked PR with a successful pipeline
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:          "Stable feature",
		Number:         2300,
		Author:         "alice",
		HeadCommitSHA:  "jkl012",
		PipelineStatus: "SUCCESS",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect PR with success status
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: Same status on next check (no change)
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should NOT send a pipeline notification
	notifs := suite.notifications.GetNotifications()
	for _, n := range notifs {
		assert.NotContains(t, n.Title, "Pipeline", "Should NOT notify when pipeline status is unchanged")
	}
}

func TestE2E_PipelineRunningToSuccess_SendsTwoNotifications(t *testing.T) {
	// Given: A tracked PR with no initial pipeline status
	suite := SetupSuite(t)
	defer suite.Teardown()

	pr := MockPR{
		Title:         "Full CI cycle",
		Number:        2400,
		Author:        "alice",
		HeadCommitSHA: "mno345",
	}
	suite.mockGitHub.SetupPRs([]MockPR{pr})

	// First check: detect PR
	err := suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.ClearNotifications()

	// Cycle 2: pipeline starts
	suite.mockGitHub.SetPipelineStatus(2400, "IN_PROGRESS")
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "Should notify on pipeline start")
	suite.ClearNotifications()

	// Cycle 3: pipeline succeeds
	suite.mockGitHub.SetPipelineStatus(2400, "SUCCESS")
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: Should notify about success transition
	notifs = suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when pipeline transitions from running to success")

	foundSuccess := false
	for _, n := range notifs {
		if n.Title == "PR Pipeline Passed 🟢" {
			foundSuccess = true
			break
		}
	}
	assert.True(t, foundSuccess, "Should send pipeline success notification after running")

	// And: Final menu state should be success
	prs := suite.menuAdapter.GetPRs()
	require.Len(t, prs, 1)
	assert.Equal(t, pullrequest.PipelineStatusSuccess, prs[0].PipelineStatus())
}

// === First-run noise suppression tests (OAK-20) ===

func TestE2E_FirstRun_NoPipelineNoise(t *testing.T) {
	// Given: Multiple PRs already exist with known pipeline statuses on first start
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{
			Title:          "Already green feature",
			Number:         3000,
			Author:         "alice",
			HeadCommitSHA:  "aaa111",
			PipelineStatus: "SUCCESS",
		},
		{
			Title:          "Failing feature",
			Number:         3001,
			Author:         "bob",
			HeadCommitSHA:  "bbb222",
			PipelineStatus: "FAILURE",
		},
		{
			Title:          "Running build",
			Number:         3002,
			Author:         "carol",
			HeadCommitSHA:  "ccc333",
			PipelineStatus: "IN_PROGRESS",
		},
	})

	// When: First-run initialisation
	err := suite.orchestrator.ExecuteInitialCheck(suite.ctx)
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: No notifications should be sent (all PRs are "already known")
	notifs := suite.notifications.GetNotifications()
	assert.Empty(t, notifs, "First run MUST NOT generate any notifications")
	suite.ClearNotifications()

	// And: Immediately running a second check with unchanged statuses also produces no noise
	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	notifs = suite.notifications.GetNotifications()
	for _, n := range notifs {
		assert.NotContains(t, n.Title, "Pipeline",
			"Second check MUST NOT re-notify for unchanged pipeline statuses")
	}
}

func TestE2E_FirstRun_PipelineChangeAfterInit_StillNotifies(t *testing.T) {
	// Given: A PR that is already known (SUCCESS) on first run
	suite := SetupSuite(t)
	defer suite.Teardown()

	suite.mockGitHub.SetupPRs([]MockPR{
		{
			Title:          "Stable feature",
			Number:         3100,
			Author:         "alice",
			HeadCommitSHA:  "ddd444",
			PipelineStatus: "SUCCESS",
		},
	})

	// First-run initialisation
	err := suite.orchestrator.ExecuteInitialCheck(suite.ctx)
	require.NoError(t, err)
	suite.ClearNotifications()

	// When: The pipeline subsequently fails
	suite.mockGitHub.SetPipelineStatus(3100, "FAILURE")

	err = suite.orchestrator.ExecuteRegularCheck(suite.ctx, time.Now())
	require.NoError(t, err)
	suite.FlushNotifications()

	// Then: We SHOULD be notified about the new failure
	notifs := suite.notifications.GetNotifications()
	require.Greater(t, len(notifs), 0, "MUST notify when pipeline changes after first run")

	foundPipeline := false
	for _, n := range notifs {
		if n.Title == "PR Pipeline Failed 🔴" {
			foundPipeline = true
			break
		}
	}
	assert.True(t, foundPipeline, "Should send pipeline failure notification after first-run init")
}
