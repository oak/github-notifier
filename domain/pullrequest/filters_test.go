package pullrequest_test

import (
	"testing"

	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestActivityIgnoreFilter_ConfigVariations(t *testing.T) {
	cfg := &pullrequest.IgnoreConfig{}
	cfg.Ignore.Global.Repos = []string{"octocat/ignored-repo"}
	cfg.Ignore.Global.Events = []string{"PipelineStatusChanged"}
	cfg.Ignore.Global.Except = []string{"PipelineStatusChanged:failed"}
	cfg.Ignore.Global.AuthoredBy = []pullrequest.IgnoreActorRule{
		{
			Login:  "renovate[bot]",
			Events: []string{"PipelineStatusChanged", "ReviewStateChanged"},
			Except: []string{"PipelineStatusChanged:failed", "ReviewStateChanged:changes_requested"},
		},
		{
			Login:  "ci-bot",
			Events: []string{"ActivityDetected"},
		},
		{
			Login:  "someuser",
			Events: []string{"ActivityDetected"},
		},
	}
	cfg.Ignore.Repos = map[string]pullrequest.IgnoreScope{
		"octocat/special-repo": {
			AuthoredBy: []pullrequest.IgnoreActorRule{
				{
					Login:  "special-bot",
					Events: []string{"Merged"},
				},
			},
		},
	}

	tests := []struct {
		desc       string
		repo       string
		event      string
		author     string
		detail     string
		wantIgnore bool
	}{
		{"ignore global repo", "octocat/ignored-repo", "NewPullRequestDetected", "any", "", true},
		{"ignore PipelineStatusChanged globally (not failed)", "any/repo", "PipelineStatusChanged", "octocat", "success", true},
		{"except PipelineStatusChanged:failed globally", "any/repo", "PipelineStatusChanged", "octocat", "failed", false},
		{"ignore PipelineStatusChanged by renovate", "any/repo", "PipelineStatusChanged", "renovate[bot]", "success", true},
		{"except PipelineStatusChanged:failed for renovate", "any/repo", "PipelineStatusChanged", "renovate[bot]", "failed", false},
		{"ignore ReviewStateChanged by renovate (not changes_requested)", "any/repo", "ReviewStateChanged", "renovate[bot]", "approved", true},
		{"except ReviewStateChanged:changes_requested for renovate", "any/repo", "ReviewStateChanged", "renovate[bot]", "changes_requested", false},
		{"ignore ActivityDetected by ci-bot", "any/repo", "ActivityDetected", "ci-bot", "comment", true},
		{"ignore ActivityDetected by someuser", "any/repo", "ActivityDetected", "someuser", "reaction", true},
		{"ignore special repo merged by special-bot", "octocat/special-repo", "Merged", "special-bot", "", true},
		{"do not ignore special repo merged by other author", "octocat/special-repo", "Merged", "octocat", "", false},
		{"do not ignore unknown author unknown event", "any/repo", "NewPullRequestDetected", "octocat", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := pullrequest.ActivityIgnoreFilter(cfg, tt.repo, tt.event, tt.author, tt.detail)
			assert.Equal(t, tt.wantIgnore, got,
				"ActivityIgnoreFilter(repo=%q, event=%q, author=%q, detail=%q)", tt.repo, tt.event, tt.author, tt.detail)
		})
	}
}

func TestNewDraftFilter_IncludeDraftsTrue(t *testing.T) {
	filter := pullrequest.NewDraftFilter(true)
	prs := testutil.CreateTestPRs(2, 3) // 2 regular, 3 drafts

	result := filter(prs)

	assert.Len(t, result, 5, "Should include all PRs when includeDrafts is true")
	testutil.AssertPRSlicesEqual(t, prs, result)
}

func TestNewDraftFilter_IncludeDraftsFalse(t *testing.T) {
	filter := pullrequest.NewDraftFilter(false)
	prs := testutil.CreateTestPRs(2, 3) // 2 regular, 3 drafts

	result := filter(prs)

	assert.Len(t, result, 2, "Should exclude draft PRs when includeDrafts is false")
	testutil.AssertNoDrafts(t, result)
}

func TestNewDraftFilter_EmptyInput(t *testing.T) {
	filter := pullrequest.NewDraftFilter(false)
	var prs []*pullrequest.PullRequest

	result := filter(prs)

	assert.Empty(t, result, "Should return empty slice for empty input")
}

func TestNewDraftFilter_OnlyDrafts(t *testing.T) {
	filter := pullrequest.NewDraftFilter(false)
	prs := testutil.CreateTestPRs(0, 3) // 0 regular, 3 drafts

	result := filter(prs)

	assert.Empty(t, result, "Should return empty slice when all PRs are drafts and includeDrafts is false")
}

func TestNewDraftFilter_OnlyRegular(t *testing.T) {
	filter := pullrequest.NewDraftFilter(false)
	prs := testutil.CreateTestPRs(3, 0) // 3 regular, 0 drafts

	result := filter(prs)

	assert.Len(t, result, 3, "Should return all PRs when none are drafts")
	testutil.AssertPRSlicesEqual(t, prs, result)
}

func TestNewDraftFilter_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		includeDrafts    bool
		regularCount     int
		draftCount       int
		expectedCount    int
		shouldHaveDrafts bool
	}{
		{"include drafts with mixed PRs", true, 2, 3, 5, true},
		{"exclude drafts with mixed PRs", false, 2, 3, 2, false},
		{"include drafts with only regular PRs", true, 5, 0, 5, false},
		{"exclude drafts with only regular PRs", false, 5, 0, 5, false},
		{"include drafts with only draft PRs", true, 0, 5, 5, true},
		{"exclude drafts with only draft PRs", false, 0, 5, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := pullrequest.NewDraftFilter(tt.includeDrafts)
			prs := testutil.CreateTestPRs(tt.regularCount, tt.draftCount)

			result := filter(prs)

			assert.Len(t, result, tt.expectedCount)

			if tt.shouldHaveDrafts {
				hasDraft := false
				for _, pr := range result {
					if pr.IsDraft() {
						hasDraft = true
						break
					}
				}
				assert.True(t, hasDraft, "Result should contain at least one draft PR")
			} else {
				testutil.AssertNoDrafts(t, result)
			}
		})
	}
}
