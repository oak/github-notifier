package pullrequest

import "time"

// Activity represents an activity/event on a pull request
type Activity struct {
	prIdentifier PRIdentifier
	activityType ActivityType
	author       Author
	createdAt    time.Time
	body         string
}

// ActivityType represents the type of activity
type ActivityType string

const (
	ActivityTypeComment  ActivityType = "comment"
	ActivityTypeReview   ActivityType = "review"
	ActivityTypeCommit   ActivityType = "commit"
	ActivityTypeReaction ActivityType = "reaction"
	ActivityTypePush     ActivityType = "push"
)

// NewActivity creates a new activity
func NewActivity(
	prIdentifier PRIdentifier,
	activityType ActivityType,
	author Author,
	createdAt time.Time,
	body string,
) *Activity {
	return &Activity{
		prIdentifier: prIdentifier,
		activityType: activityType,
		author:       author,
		createdAt:    createdAt,
		body:         body,
	}
}

// PRIdentifier returns the PR identifier
func (a *Activity) PRIdentifier() PRIdentifier {
	return a.prIdentifier
}

// Type returns the activity type
func (a *Activity) Type() ActivityType {
	return a.activityType
}

// Author returns the activity author
func (a *Activity) Author() Author {
	return a.author
}

// CreatedAt returns when the activity was created
func (a *Activity) CreatedAt() time.Time {
	return a.createdAt
}

// Body returns the activity body/content
func (a *Activity) Body() string {
	return a.body
}
