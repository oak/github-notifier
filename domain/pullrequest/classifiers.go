package pullrequest

import "time"

// ClassifyPRs separates PRs into two categories:
// - trulyNew: Brand new PRs without recent activity
// - withActivity: PRs that have activity since the given time
//
// This classification is used to determine notification messages and tracking semantics
func ClassifyPRs(prs []*PullRequest, since time.Time) (trulyNew, withActivity []*PullRequest) {
	trulyNew = make([]*PullRequest, 0)
	withActivity = make([]*PullRequest, 0)

	for _, pr := range prs {
		if pr.HasActivitiesSince(since) {
			withActivity = append(withActivity, pr)
		} else {
			trulyNew = append(trulyNew, pr)
		}
	}

	return trulyNew, withActivity
}
