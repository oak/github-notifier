package pullrequest

import "time"

// PRClassifier classifies pull requests based on their state and activity
type PRClassifier struct{}

// NewPRClassifier creates a new PR classifier
func NewPRClassifier() *PRClassifier {
	return &PRClassifier{}
}

// ClassifyPRs separates PRs into two categories:
// - trulyNew: Brand new PRs without recent activity
// - withActivity: PRs that have activity since the given time
//
// This classification is used to determine notification messages and tracking semantics
func (c *PRClassifier) ClassifyPRs(prs []*PullRequest, since time.Time) (trulyNew, withActivity []*PullRequest) {
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
