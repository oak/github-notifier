package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// AssertPREquals verifies two PRs are equal
func AssertPREquals(t *testing.T, expected, actual *pullrequest.PullRequest) {
	t.Helper()
	assert.Equal(t, expected.URL(), actual.URL(), "PR URLs should match")
	assert.Equal(t, expected.Number(), actual.Number(), "PR numbers should match")
	assert.Equal(t, expected.Title(), actual.Title(), "PR titles should match")
	assert.Equal(t, expected.RepositoryName(), actual.RepositoryName(), "PR repositories should match")
	assert.Equal(t, expected.AuthorLogin(), actual.AuthorLogin(), "PR authors should match")
	assert.Equal(t, expected.IsDraft(), actual.IsDraft(), "PR draft status should match")
}

// AssertPRSlicesEqual verifies two PR slices contain the same PRs
func AssertPRSlicesEqual(t *testing.T, expected, actual []*pullrequest.PullRequest) {
	t.Helper()
	assert.Equal(t, len(expected), len(actual), "PR slice lengths should match")
	for i := range expected {
		AssertPREquals(t, expected[i], actual[i])
	}
}

// AssertActivityEquals verifies two activities are equal
func AssertActivityEquals(t *testing.T, expected, actual *pullrequest.Activity) {
	t.Helper()
	assert.Equal(t, expected.Type(), actual.Type(), "Activity types should match")
	assert.Equal(t, expected.Author().Login(), actual.Author().Login(), "Activity authors should match")
	assert.Equal(t, expected.Body(), actual.Body(), "Activity bodies should match")
	assert.True(t, expected.CreatedAt().Equal(actual.CreatedAt()), "Activity creation times should match")
}

// AssertContainsPR verifies a PR slice contains a specific PR
func AssertContainsPR(t *testing.T, slice []*pullrequest.PullRequest, pr *pullrequest.PullRequest) {
	t.Helper()
	found := false
	for _, p := range slice {
		if p.URL() == pr.URL() && p.Number() == pr.Number() {
			found = true
			break
		}
	}
	assert.True(t, found, "PR slice should contain PR %s #%d", pr.URL(), pr.Number())
}

// AssertNotContainsPR verifies a PR slice does not contain a specific PR
func AssertNotContainsPR(t *testing.T, slice []*pullrequest.PullRequest, pr *pullrequest.PullRequest) {
	t.Helper()
	for _, p := range slice {
		if p.URL() == pr.URL() && p.Number() == pr.Number() {
			t.Errorf("PR slice should not contain PR %s #%d", pr.URL(), pr.Number())
		}
	}
}

// AssertAllDrafts verifies all PRs in a slice are drafts
func AssertAllDrafts(t *testing.T, prs []*pullrequest.PullRequest) {
	t.Helper()
	for _, pr := range prs {
		assert.True(t, pr.IsDraft(), "PR %s should be a draft", pr.URL())
	}
}

// AssertNoDrafts verifies no PRs in a slice are drafts
func AssertNoDrafts(t *testing.T, prs []*pullrequest.PullRequest) {
	t.Helper()
	for _, pr := range prs {
		assert.False(t, pr.IsDraft(), "PR %s should not be a draft", pr.URL())
	}
}
