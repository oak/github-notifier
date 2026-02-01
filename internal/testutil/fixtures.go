package testutil

import (
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// PROption allows functional options pattern for test PR creation
type PROption func(*pullrequest.PullRequest)

// NewTestRepository creates a test repository
func NewTestRepository(nameWithOwner string) pullrequest.RepositoryInfo {
	repo, err := pullrequest.NewRepository(nameWithOwner)
	if err != nil {
		panic(err)
	}
	return repo
}

// NewTestAuthor creates a test author
func NewTestAuthor(login string) pullrequest.Author {
	author, err := pullrequest.NewAuthor(login)
	if err != nil {
		panic(err)
	}
	return author
}

// NewTestPullRequest creates a test pull request with sensible defaults
func NewTestPullRequest(number int, opts ...func(*testPRBuilder)) *pullrequest.PullRequest {
	builder := &testPRBuilder{
		url:       "https://github.com/owner/repo/pull/1",
		number:    number,
		title:     "Test PR",
		repo:      NewTestRepository("owner/repo"),
		author:    NewTestAuthor("testuser"),
		createdAt: time.Now(),
		isDraft:   false,
	}

	for _, opt := range opts {
		opt(builder)
	}

	pr, err := pullrequest.NewPullRequest(
		builder.url,
		builder.number,
		builder.title,
		builder.repo,
		builder.author,
		builder.createdAt,
		builder.isDraft,
	)
	if err != nil {
		panic(err)
	}

	return pr
}

type testPRBuilder struct {
	url       string
	number    int
	title     string
	repo      pullrequest.RepositoryInfo
	author    pullrequest.Author
	createdAt time.Time
	isDraft   bool
}

// WithURL sets the PR URL
func WithURL(url string) func(*testPRBuilder) {
	return func(b *testPRBuilder) {
		b.url = url
	}
}

// WithTitle sets the PR title
func WithTitle(title string) func(*testPRBuilder) {
	return func(b *testPRBuilder) {
		b.title = title
	}
}

// WithRepository sets the PR repository
func WithRepository(nameWithOwner string) func(*testPRBuilder) {
	return func(b *testPRBuilder) {
		b.repo = NewTestRepository(nameWithOwner)
	}
}

// WithAuthor sets the PR author
func WithAuthor(login string) func(*testPRBuilder) {
	return func(b *testPRBuilder) {
		b.author = NewTestAuthor(login)
	}
}

// WithCreatedAt sets the PR creation time
func WithCreatedAt(t time.Time) func(*testPRBuilder) {
	return func(b *testPRBuilder) {
		b.createdAt = t
	}
}

// WithDraft sets the PR draft status
func WithDraft(isDraft bool) func(*testPRBuilder) {
	return func(b *testPRBuilder) {
		b.isDraft = isDraft
	}
}

// NewTestActivity creates a test activity
func NewTestActivity(activityType pullrequest.ActivityType, createdAt time.Time, opts ...func(*testActivityBuilder)) *pullrequest.Activity {
	prIdentifier, err := pullrequest.NewPRIdentifier("https://github.com/owner/repo/pull/1", 1)
	if err != nil {
		panic(err)
	}

	builder := &testActivityBuilder{
		prIdentifier: prIdentifier,
		activityType: activityType,
		author:       NewTestAuthor("testuser"),
		createdAt:    createdAt,
		body:         "Test activity",
	}

	for _, opt := range opts {
		opt(builder)
	}

	return pullrequest.NewActivity(
		builder.prIdentifier,
		builder.activityType,
		builder.author,
		builder.createdAt,
		builder.body,
	)
}

type testActivityBuilder struct {
	prIdentifier pullrequest.PRIdentifier
	activityType pullrequest.ActivityType
	author       pullrequest.Author
	createdAt    time.Time
	body         string
}

// WithActivityPR sets the PR identifier for the activity
func WithActivityPR(url string, number int) func(*testActivityBuilder) {
	return func(b *testActivityBuilder) {
		prIdentifier, err := pullrequest.NewPRIdentifier(url, number)
		if err != nil {
			panic(err)
		}
		b.prIdentifier = prIdentifier
	}
}

// WithActivityAuthor sets the activity author
func WithActivityAuthor(login string) func(*testActivityBuilder) {
	return func(b *testActivityBuilder) {
		b.author = NewTestAuthor(login)
	}
}

// WithActivityBody sets the activity body
func WithActivityBody(body string) func(*testActivityBuilder) {
	return func(b *testActivityBuilder) {
		b.body = body
	}
}

// CreateTestPRs creates multiple test PRs with specified draft counts
func CreateTestPRs(regularCount, draftCount int) []*pullrequest.PullRequest {
	prs := make([]*pullrequest.PullRequest, 0, regularCount+draftCount)

	// Create regular PRs
	for i := 0; i < regularCount; i++ {
		pr := NewTestPullRequest(i+1, WithDraft(false))
		prs = append(prs, pr)
	}

	// Create draft PRs
	for i := 0; i < draftCount; i++ {
		pr := NewTestPullRequest(regularCount+i+1, WithDraft(true))
		prs = append(prs, pr)
	}

	return prs
}

// CreateTestPRsWithActivities creates test PRs with activities
func CreateTestPRsWithActivities(count int, activitiesPerPR int, activityAge time.Duration) []*pullrequest.PullRequest {
	prs := make([]*pullrequest.PullRequest, 0, count)
	now := time.Now()

	for i := 0; i < count; i++ {
		pr := NewTestPullRequest(i + 1)

		// Add activities
		activities := make([]*pullrequest.Activity, 0, activitiesPerPR)
		for j := 0; j < activitiesPerPR; j++ {
			activity := NewTestActivity(
				pullrequest.ActivityTypeComment,
				now.Add(-activityAge),
				WithActivityPR(pr.URL(), pr.Number()),
			)
			activities = append(activities, activity)
		}

		pr.AddActivities(activities)
		prs = append(prs, pr)
	}

	return prs
}
