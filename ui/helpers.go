package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/oak3/github-notifier/domain"
)

// groupPRsByRepository groups PRs by their repository
func groupPRsByRepository(prs []domain.PullRequest) map[string][]domain.PullRequest {
	grouped := make(map[string][]domain.PullRequest)
	for _, pr := range prs {
		grouped[pr.Repository.NameWithOwner] = append(grouped[pr.Repository.NameWithOwner], pr)
	}
	return grouped
}

// SortPRsByCreatedAt sorts PRs by creation time (oldest to newest)
func SortPRsByCreatedAt(prs []domain.PullRequest) {
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].CreatedAt.Before(prs[j].CreatedAt)
	})
}

// formatPRTitle returns a formatted PR title with age information
func formatPRTitle(pr domain.PullRequest) string {
	return fmt.Sprintf("[%s] %s", formatTimeAgo(pr.CreatedAt), pr.Title)
}

// formatTimeAgo returns a human-readable time difference
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	hours := duration.Hours()
	days := hours / 24
	weeks := int(days / 7)

	if weeks > 0 {
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if days >= 2 {
		return fmt.Sprintf("%d days ago", int(days))
	} else {
		return fmt.Sprintf("%d hours ago", int(hours))
	}
}
