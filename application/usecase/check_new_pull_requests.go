package usecase

import (
	"log"
	"time"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/domain/tracking"
)

// CheckNewPullRequestsUseCase handles fetching and detecting new PRs
// Emits domain events for new PRs instead of directly sending notifications
type CheckNewPullRequestsUseCase struct {
	prRepo          pullrequest.PullRequestRepository
	trackingService tracking.Service
	prFilter        *pullrequest.PRFilter
	prClassifier    *pullrequest.PRClassifier
	eventPublisher  port.EventPublisher
	lastCheckTime   time.Time
}

// NewCheckNewPullRequestsUseCase creates a new use case
func NewCheckNewPullRequestsUseCase(
	prRepo pullrequest.PullRequestRepository,
	trackingService tracking.Service,
	prFilter *pullrequest.PRFilter,
	prClassifier *pullrequest.PRClassifier,
	eventPublisher port.EventPublisher,
) *CheckNewPullRequestsUseCase {
	return &CheckNewPullRequestsUseCase{
		prRepo:          prRepo,
		trackingService: trackingService,
		prFilter:        prFilter,
		prClassifier:    prClassifier,
		eventPublisher:  eventPublisher,
		lastCheckTime:   time.Now(),
	}
}

// PRCheckResult contains the results of checking for new PRs
type PRCheckResult struct {
	RequestedReviewPRs []*pullrequest.PullRequest
	UserCreatedPRs     []*pullrequest.PullRequest
}

// Execute fetches PRs and detects new ones
// Returns the fetched PRs for use by other use cases (activity tracking, display)
func (uc *CheckNewPullRequestsUseCase) Execute() (*PRCheckResult, error) {
	// Fetch PRs from both sources
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Printf("Error fetching requested review PRs: %v", err)
		return nil, err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Printf("Error fetching user created PRs: %v", err)
		return nil, err
	}

	// Filter draft PRs if configured
	requestedReviewPRs = uc.prFilter.FilterDrafts(requestedReviewPRs)
	userCreatedPRs = uc.prFilter.FilterDrafts(userCreatedPRs)

	// Process requested review PRs
	if err := uc.processNewPRs(requestedReviewPRs, "requested review"); err != nil {
		log.Printf("Error processing requested review PRs: %v", err)
	}

	// Process user created PRs
	if err := uc.processNewPRs(userCreatedPRs, "user created"); err != nil {
		log.Printf("Error processing user created PRs: %v", err)
	}

	// Update last check time
	uc.lastCheckTime = time.Now()

	return &PRCheckResult{
		RequestedReviewPRs: requestedReviewPRs,
		UserCreatedPRs:     userCreatedPRs,
	}, nil
}

// processNewPRs finds new PRs and emits appropriate events
func (uc *CheckNewPullRequestsUseCase) processNewPRs(prs []*pullrequest.PullRequest, category string) error {
	// Find PRs that are new (not seen in tracking service)
	newPRs := uc.trackingService.FindNewPullRequests(prs)

	if len(newPRs) == 0 {
		return nil
	}

	log.Printf("Found %d new %s PRs", len(newPRs), category)

	// Classify PRs: truly new vs. PRs with new activity
	trulyNewPRs, prsWithActivity := uc.prClassifier.ClassifyPRs(newPRs, uc.lastCheckTime)

	// Emit events for truly new PRs
	for _, pr := range trulyNewPRs {
		event := pullrequest.NewNewPullRequestDetected(pr)
		if err := uc.eventPublisher.Publish(&event); err != nil {
			log.Printf("Error publishing new PR event: %v", err)
		}
	}

	// Mark truly new PRs as seen (for notification purposes)
	// They'll still show asterisks until user clicks
	if len(trulyNewPRs) > 0 {
		uc.trackingService.MarkPullRequestsAsSeen(trulyNewPRs)
	}

	// PRs with activity remain unseen and will trigger activity events
	// They're handled by the TrackPullRequestActivityUseCase
	if len(prsWithActivity) > 0 {
		log.Printf("%d PRs have new activity (handled separately)", len(prsWithActivity))
	}

	return nil
}
