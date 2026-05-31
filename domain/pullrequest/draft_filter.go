package pullrequest

// FilterFn is a pure function from PR slice → filtered PR slice.
type FilterFn func([]*PullRequest) []*PullRequest

// NewDraftFilter returns the appropriate FilterFn based on configuration.
// When includeDrafts is true, the returned function is a no-op passthrough.
// When includeDrafts is false, the returned function strips all draft PRs.
func NewDraftFilter(includeDrafts bool) FilterFn {
	if includeDrafts {
		return func(prs []*PullRequest) []*PullRequest { return prs }
	}
	return func(prs []*PullRequest) []*PullRequest {
		out := make([]*PullRequest, 0, len(prs))
		for _, pr := range prs {
			if !pr.IsDraft() {
				out = append(out, pr)
			}
		}
		return out
	}
}
