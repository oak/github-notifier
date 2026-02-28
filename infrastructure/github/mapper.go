package github

import (
	"fmt"
	"time"

	"github.com/oak3/github-notifier/domain/pullrequest"
)

// Mapper converts between DTOs and domain models
type Mapper struct{}

// NewMapper creates a new mapper
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToDomain converts a GitHub DTO to a domain PullRequest
func (m *Mapper) ToDomain(dto PullRequestDTO) (*pullrequest.PullRequest, error) {
	repo, err := pullrequest.NewRepository(dto.Repository.NameWithOwner)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	author, err := pullrequest.NewAuthor(dto.Author.Login)
	if err != nil {
		return nil, fmt.Errorf("failed to create author: %w", err)
	}

	pr, err := pullrequest.NewPullRequest(
		dto.URL,
		dto.Number,
		dto.Title,
		repo,
		author,
		dto.CreatedAt,
		dto.IsDraft,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	// Map reviews if present
	if dto.LatestReviews != nil {
		reviews := m.ToReviews(dto.LatestReviews.Nodes)
		// Set initial reviews without raising events (this is initial state, not a change)
		pr.SetInitialReviews(reviews)
	}

	// Map initial pipeline status if present
	if dto.Commits != nil && len(dto.Commits.Nodes) > 0 {
		last := dto.Commits.Nodes[len(dto.Commits.Nodes)-1]
		if last.Commit.StatusCheckRollup != nil {
			status := pullrequest.PipelineStatusFromRollup(last.Commit.StatusCheckRollup.State)
			// Set initial pipeline status without raising events (this is initial state, not a change)
			pr.SetInitialPipelineStatus(status)
		}
	}

	return pr, nil
}

// ToDomainList converts a list of DTOs to domain PullRequests
func (m *Mapper) ToDomainList(dtos []PullRequestDTO) ([]*pullrequest.PullRequest, error) {
	prs := make([]*pullrequest.PullRequest, 0, len(dtos))

	for _, dto := range dtos {
		pr, err := m.ToDomain(dto)
		if err != nil {
			// Log error but continue with other PRs
			continue
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

// ToReviews converts a list of ReviewDTOs to a map of reviewer login -> latest Review.
// GitHub's latestReviews connection returns the most recent review per reviewer,
// so we map each one directly.
func (m *Mapper) ToReviews(dtos []ReviewDTO) map[string]*pullrequest.Review {
	reviews := make(map[string]*pullrequest.Review)

	for _, dto := range dtos {
		if dto.Author.Login == "" {
			continue
		}

		state, ok := pullrequest.ReviewStateFromString(dto.State)
		if !ok {
			continue
		}

		author, err := pullrequest.NewAuthor(dto.Author.Login)
		if err != nil {
			continue
		}

		reviews[dto.Author.Login] = pullrequest.NewReview(author, state, dto.SubmittedAt)
	}

	return reviews
}

// ToActivity converts a single timeline item DTO to a domain activity
func (m *Mapper) ToActivity(pr *pullrequest.PullRequest, dto TimelineItemDTO) *pullrequest.Activity {
	switch dto.Typename {
	case "IssueComment":
		if dto.Author == nil {
			return nil
		}
		author, err := pullrequest.NewAuthor(dto.Author.Login)
		if err != nil {
			return nil
		}
		return pullrequest.NewActivity(
			pr.Identifier(),
			pullrequest.ActivityTypeComment,
			author,
			dto.CreatedAt,
			dto.Body,
		)

	case "PullRequestReview":
		if dto.Author == nil {
			return nil
		}
		// Only notify on reviews with comments or approval/changes requested
		if dto.Body != "" || dto.State == "APPROVED" || dto.State == "CHANGES_REQUESTED" {
			author, err := pullrequest.NewAuthor(dto.Author.Login)
			if err != nil {
				return nil
			}
			return pullrequest.NewActivity(
				pr.Identifier(),
				pullrequest.ActivityTypeReview,
				author,
				dto.CreatedAt,
				dto.Body,
			)
		}

	case "PullRequestCommit":
		if dto.Commit != nil {
			// For commits, use the commit date and author from nested structure
			authorLogin := "unknown"
			if dto.Commit.Author != nil && dto.Commit.Author.User != nil {
				authorLogin = dto.Commit.Author.User.Login
			}

			author, err := pullrequest.NewAuthor(authorLogin)
			if err != nil {
				return nil
			}
			return pullrequest.NewActivity(
				pr.Identifier(),
				pullrequest.ActivityTypeCommit,
				author,
				dto.Commit.CommittedDate,
				dto.Commit.OID[:7], // Use short commit SHA as body
			)
		}
	}

	return nil
}

// ToActivityList converts timeline item DTOs to domain activities, filtering by time.
// All activities are recorded as domain facts; notification filtering (e.g. suppressing
// self-authored activities) is handled by the notification event handler.
func (m *Mapper) ToActivityList(pr *pullrequest.PullRequest, dtos []TimelineItemDTO, since time.Time) []*pullrequest.Activity {
	var activities []*pullrequest.Activity

	for _, dto := range dtos {
		// Check for reactions on this timeline item (even if the item itself is old)
		reactionActivities := m.ToReactionActivities(pr, dto, since)
		activities = append(activities, reactionActivities...)

		// Skip items older than since time for regular activities
		if dto.CreatedAt.Before(since) {
			continue
		}

		activity := m.ToActivity(pr, dto)
		if activity != nil {
			activities = append(activities, activity)
		}
	}

	return activities
}

// ToReactionActivities extracts reaction activities from a timeline item (comment or review).
// All reactions are recorded as domain facts; notification filtering (e.g. suppressing
// self-reactions) is handled by the notification event handler.
func (m *Mapper) ToReactionActivities(pr *pullrequest.PullRequest, dto TimelineItemDTO, since time.Time) []*pullrequest.Activity {
	var activities []*pullrequest.Activity

	// Only process reactions for comments and reviews
	if dto.Typename != "IssueComment" && dto.Typename != "PullRequestReview" {
		return activities
	}

	// Check if reactions exist
	if dto.Reactions == nil || len(dto.Reactions.Nodes) == 0 {
		return activities
	}

	// Convert each reaction to an activity
	for _, reaction := range dto.Reactions.Nodes {
		// Skip reactions older than since time
		if reaction.CreatedAt.Before(since) {
			continue
		}

		// Skip if no user info
		if reaction.User == nil {
			continue
		}

		author, err := pullrequest.NewAuthor(reaction.User.Login)
		if err != nil {
			continue
		}

		// Create reaction activity with emoji as body
		activity := pullrequest.NewActivity(
			pr.Identifier(),
			pullrequest.ActivityTypeReaction,
			author,
			reaction.CreatedAt,
			reaction.Content, // Emoji like THUMBS_UP, HEART, etc.
		)

		activities = append(activities, activity)
	}

	return activities
}
