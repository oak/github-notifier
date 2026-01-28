package usecase

import (
	"log"
	"sort"
	"time"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
	"github.com/oak3/github-notifier/domain/tracking"
)

// CheckPullRequestsUseCase handles checking for pull requests and updating the UI
type CheckPullRequestsUseCase struct {
	prRepo                 pullrequest.PullRequestRepository
	trackingService        tracking.Service
	notificationPort       port.NotificationPort
	menuPort               port.MenuPort
	enableActivityTracking bool
	includeDraftPRs        bool
	lastCheckTime          time.Time
	recentPRThreshold      time.Duration
	stalePRCheckInterval   time.Duration
	lastActivityCheckMap   map[string]time.Time // Tracks when each PR was last checked for activities
}

// NewCheckPullRequestsUseCase creates a new use case
func NewCheckPullRequestsUseCase(
	prRepo pullrequest.PullRequestRepository,
	trackingService tracking.Service,
	notificationPort port.NotificationPort,
	menuPort port.MenuPort,
	enableActivityTracking bool,
	includeDraftPRs bool,
	recentThresholdHours int,
	staleCheckIntervalMin int,
) *CheckPullRequestsUseCase {
	return &CheckPullRequestsUseCase{
		prRepo:                 prRepo,
		trackingService:        trackingService,
		notificationPort:       notificationPort,
		menuPort:               menuPort,
		enableActivityTracking: enableActivityTracking,
		includeDraftPRs:        includeDraftPRs,
		lastCheckTime:          time.Now(), // Initialize to now
		recentPRThreshold:      time.Duration(recentThresholdHours) * time.Hour,
		stalePRCheckInterval:   time.Duration(staleCheckIntervalMin) * time.Minute,
		lastActivityCheckMap:   make(map[string]time.Time),
	}
}

// ExecuteInitial runs the use case on app startup
// If this is the very first run (seen repository is empty), mark all PRs as seen
// to avoid notifications and asterisks for PRs that existed before the app started.
// If the repository has data (e.g., Postgres with persistent state), behave like Execute()
func (uc *CheckPullRequestsUseCase) ExecuteInitial() error {
	// Check if this is truly the first run ever (seen repository is empty)
	isFirstRunEver := uc.trackingService.IsEmpty()

	// Fetch pull requests
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Printf("Error fetching requested review PRs: %v", err)
		return err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Printf("Error fetching user created PRs: %v", err)
		return err
	}

	// Filter draft PRs if configured
	requestedReviewPRs = uc.filterDraftPRs(requestedReviewPRs)
	userCreatedPRs = uc.filterDraftPRs(userCreatedPRs)

	// Sort PRs by creation date (oldest first)
	uc.sortPRsByCreatedAt(requestedReviewPRs)
	uc.sortPRsByCreatedAt(userCreatedPRs)

	if isFirstRunEver {
		// First run ever: mark all existing PRs as seen (no notifications, no asterisks)
		log.Println("First run detected - marking all existing PRs as already seen")
		uc.trackingService.MarkPullRequestsAsSeen(requestedReviewPRs)
		uc.trackingService.MarkPullRequestsAsSeen(userCreatedPRs)

		// Update the menu with tracking service
		uc.menuPort.UpdateMenu(requestedReviewPRs, userCreatedPRs, uc.trackingService)
	} else {
		// Not first run: repository has data, behave normally
		log.Println("Existing state detected - checking for new PRs")

		// Update the menu first
		uc.menuPort.UpdateMenu(requestedReviewPRs, userCreatedPRs, uc.trackingService)

		// Track and notify new PRs (same logic as Execute)
		newRequestedReviewPRs := uc.trackingService.FindNewPullRequests(requestedReviewPRs)
		if len(newRequestedReviewPRs) > 0 {
			err := uc.notificationPort.NotifyNewPullRequests("New PRs needing review", newRequestedReviewPRs)
			if err != nil {
				log.Printf("Error sending notification for requested review PRs: %v", err)
			}
			uc.trackingService.MarkPullRequestsAsSeen(newRequestedReviewPRs)
		}

		newUserCreatedPRs := uc.trackingService.FindNewPullRequests(userCreatedPRs)
		if len(newUserCreatedPRs) > 0 {
			err := uc.notificationPort.NotifyNewPullRequests("New PRs by you", newUserCreatedPRs)
			if err != nil {
				log.Printf("Error sending notification for user created PRs: %v", err)
			}
			uc.trackingService.MarkPullRequestsAsSeen(newUserCreatedPRs)
		}
	}

	return nil
}

// Execute runs the use case
func (uc *CheckPullRequestsUseCase) Execute() error {
	// Fetch pull requests
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Printf("Error fetching requested review PRs: %v", err)
		return err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Printf("Error fetching user created PRs: %v", err)
		return err
	}

	// Filter draft PRs if configured
	requestedReviewPRs = uc.filterDraftPRs(requestedReviewPRs)
	userCreatedPRs = uc.filterDraftPRs(userCreatedPRs)

	// Check for activity if enabled with two-tier optimization
	if uc.enableActivityTracking {
		// Combine all PRs for activity checking
		allPRs := append([]*pullrequest.PullRequest{}, requestedReviewPRs...)
		allPRs = append(allPRs, userCreatedPRs...)

		// Filter PRs that need activity checking (two-tier logic)
		var prsToCheck []*pullrequest.PullRequest
		var countRecent, countStale int
		now := time.Now()

		for _, pr := range allPRs {
			prURL := pr.URL()
			prAge := now.Sub(pr.CreatedAt())
			lastCheck := uc.lastActivityCheckMap[prURL]

			// Determine if this PR should be checked
			shouldCheck := false
			if prAge < uc.recentPRThreshold {
				// Recent PR: always check
				shouldCheck = true
				countRecent++
			} else {
				// Stale PR: check if enough time passed since last check
				timeSinceLastCheck := now.Sub(lastCheck)
				if timeSinceLastCheck >= uc.stalePRCheckInterval {
					shouldCheck = true
					countStale++
				}
			}

			if shouldCheck {
				prsToCheck = append(prsToCheck, pr)
			}
		}

		skipped := len(allPRs) - len(prsToCheck)
		log.Printf("Activity tracking: checking %d/%d PRs (%d recent < %dh, %d stale due for check, %d skipped)",
			len(prsToCheck), len(allPRs), countRecent,
			int(uc.recentPRThreshold.Hours()), countStale, skipped)

		// Enrich only the filtered PRs with activities since last check
		if len(prsToCheck) > 0 {
			if err := uc.prRepo.EnrichWithActivities(prsToCheck, uc.lastCheckTime); err != nil {
				log.Printf("Error enriching PRs with activities: %v", err)
			}

			// Update lastActivityCheckMap for all checked PRs
			for _, pr := range prsToCheck {
				uc.lastActivityCheckMap[pr.URL()] = now
			}
		}

		// Check for PRs with new activities and mark them as unseen
		var prsWithNewActivity []*pullrequest.PullRequest
		for _, pr := range prsToCheck {
			if pr.HasActivitiesSince(uc.lastCheckTime) {
				prsWithNewActivity = append(prsWithNewActivity, pr)
			}
		}

		if len(prsWithNewActivity) > 0 {
			log.Printf("Found %d PRs with new activity", len(prsWithNewActivity))
			// Mark these PRs as unseen so they trigger notifications and show asterisks
			for _, pr := range prsWithNewActivity {
				if err := uc.trackingService.MarkPullRequestAsUnseen(pr); err != nil {
					log.Printf("Error unmarking PR as seen: %v", err)
				}
			}
		}
	} else {
		log.Printf("Activity tracking disabled - only tracking new PRs (set ENABLE_ACTIVITY_TRACKING=true to enable)")
	}

	// Sort PRs by creation date (oldest first)
	uc.sortPRsByCreatedAt(requestedReviewPRs)
	uc.sortPRsByCreatedAt(userCreatedPRs)

	// Find PRs that need notifications (not seen in tracking service)
	// These include: truly new PRs AND PRs we just unmarked due to new activity
	newRequestedReviewPRs := uc.trackingService.FindNewPullRequests(requestedReviewPRs)
	if len(newRequestedReviewPRs) > 0 {
		// Separate truly new PRs from PRs with new activity
		var trulyNewPRs []*pullrequest.PullRequest
		var prsWithActivity []*pullrequest.PullRequest

		for _, pr := range newRequestedReviewPRs {
			if pr.HasActivitiesSince(uc.lastCheckTime) {
				prsWithActivity = append(prsWithActivity, pr)
			} else {
				trulyNewPRs = append(trulyNewPRs, pr)
			}
		}

		// Send appropriate notifications
		if len(trulyNewPRs) > 0 {
			err := uc.notificationPort.NotifyNewPullRequests("New PRs needing review", trulyNewPRs)
			if err != nil {
				log.Printf("Error sending notification for new PRs: %v", err)
			}
			// Mark truly new PRs as seen for notification purposes
			// But they stay unseen for menu (user must click)
			uc.trackingService.MarkPullRequestsAsSeen(trulyNewPRs)
		}

		if len(prsWithActivity) > 0 {
			err := uc.notificationPort.NotifyNewPullRequests("New activity on PRs needing review", prsWithActivity)
			if err != nil {
				log.Printf("Error sending notification for PR activity: %v", err)
			}
			// DON'T mark as seen - keep them unseen so asterisks persist until user clicks
			// They were already marked as unseen earlier, so notifications will repeat
			// unless we track this differently. For now, accept repeat notifications
			// or user can click to acknowledge
		}
	}

	newUserCreatedPRs := uc.trackingService.FindNewPullRequests(userCreatedPRs)
	if len(newUserCreatedPRs) > 0 {
		// Separate truly new PRs from PRs with new activity
		var trulyNewPRs []*pullrequest.PullRequest
		var prsWithActivity []*pullrequest.PullRequest

		for _, pr := range newUserCreatedPRs {
			if pr.HasActivitiesSince(uc.lastCheckTime) {
				prsWithActivity = append(prsWithActivity, pr)
			} else {
				trulyNewPRs = append(trulyNewPRs, pr)
			}
		}

		// Send appropriate notifications
		if len(trulyNewPRs) > 0 {
			err := uc.notificationPort.NotifyNewPullRequests("New PRs by you", trulyNewPRs)
			if err != nil {
				log.Printf("Error sending notification for new PRs: %v", err)
			}
			// Mark truly new PRs as seen for notification purposes
			uc.trackingService.MarkPullRequestsAsSeen(trulyNewPRs)
		}

		if len(prsWithActivity) > 0 {
			err := uc.notificationPort.NotifyNewPullRequests("New activity on your PRs", prsWithActivity)
			if err != nil {
				log.Printf("Error sending notification for PR activity: %v", err)
			}
			// DON'T mark as seen - keep them unseen so asterisks persist until user clicks
		}
	}

	// Update the menu AFTER all notification logic completes
	// This ensures PRs with new activity remain unseen (show asterisks)
	uc.menuPort.UpdateMenu(requestedReviewPRs, userCreatedPRs, uc.trackingService)

	// Update last check time for next iteration
	uc.lastCheckTime = time.Now()

	return nil
}

// sortPRsByCreatedAt sorts pull requests by creation date (oldest first)
func (uc *CheckPullRequestsUseCase) sortPRsByCreatedAt(prs []*pullrequest.PullRequest) {
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].CreatedAt().Before(prs[j].CreatedAt())
	})
}

// filterDraftPRs filters out draft PRs if configured to do so
func (uc *CheckPullRequestsUseCase) filterDraftPRs(prs []*pullrequest.PullRequest) []*pullrequest.PullRequest {
	if uc.includeDraftPRs {
		return prs
	}

	filtered := make([]*pullrequest.PullRequest, 0, len(prs))
	for _, pr := range prs {
		if !pr.IsDraft() {
			filtered = append(filtered, pr)
		}
	}
	return filtered
}
