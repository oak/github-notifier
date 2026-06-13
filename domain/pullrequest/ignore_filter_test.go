package pullrequest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/oak/github-notifier/domain/pullrequest"
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
