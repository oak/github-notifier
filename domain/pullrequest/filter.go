package pullrequest

// PRFilter provides filtering capabilities for pull requests
type PRFilter struct {
	includeDrafts bool
}

// NewPRFilter creates a new PR filter
func NewPRFilter(includeDrafts bool) *PRFilter {
	return &PRFilter{
		includeDrafts: includeDrafts,
	}
}

// FilterDrafts filters out draft PRs if configured to do so
// Returns a new slice with non-draft PRs only (if includeDrafts is false)
// Otherwise returns the original slice unchanged
func (f *PRFilter) FilterDrafts(prs []*PullRequest) []*PullRequest {
	if f.includeDrafts {
		return prs
	}

	filtered := make([]*PullRequest, 0, len(prs))
	for _, pr := range prs {
		if !pr.IsDraft() {
			filtered = append(filtered, pr)
		}
	}
	return filtered
}
