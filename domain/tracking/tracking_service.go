package tracking

import (
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// TrackingService implements the Service interface
type TrackingService struct {
	seenRepo SeenRepository
}

// NewTrackingService creates a new tracking service
func NewTrackingService(seenRepo SeenRepository) *TrackingService {
	return &TrackingService{
		seenRepo: seenRepo,
	}
}

// TrackPullRequest tracks a PR and returns true if it's new (not seen before)
func (s *TrackingService) TrackPullRequest(pr *pullrequest.PullRequest) bool {
	id := pr.Identifier()

	if s.seenRepo.HasBeenSeen(id) {
		return false
	}

	s.seenRepo.MarkAsSeen(id)
	return true
}

// HasBeenSeen checks if a PR has been seen before
func (s *TrackingService) HasBeenSeen(id pullrequest.PRIdentifier) bool {
	return s.seenRepo.HasBeenSeen(id)
}

// FindNewPullRequests identifies which PRs in the list are new (without marking them as seen)
func (s *TrackingService) FindNewPullRequests(prs []*pullrequest.PullRequest) []*pullrequest.PullRequest {
	var newPRs []*pullrequest.PullRequest

	for _, pr := range prs {
		if !s.seenRepo.HasBeenSeen(pr.Identifier()) {
			newPRs = append(newPRs, pr)
		}
	}

	return newPRs
}

// MarkPullRequestsAsSeen marks a list of PRs as seen
func (s *TrackingService) MarkPullRequestsAsSeen(prs []*pullrequest.PullRequest) {
	for _, pr := range prs {
		s.seenRepo.MarkAsSeen(pr.Identifier())
	}
}

// IsEmpty returns true if no PRs have been tracked yet
func (s *TrackingService) IsEmpty() bool {
	return s.seenRepo.IsEmpty()
}
