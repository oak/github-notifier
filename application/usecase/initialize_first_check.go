package usecase

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/application/port"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// InitializeFirstCheckUseCase handles the first-run initialization
// On first run, all existing PRs are marked as seen to avoid notifications
type InitializeFirstCheckUseCase struct {
	prRepo         pullrequest.PullRequestRepository
	prTrackingRepo pullrequest.PRTrackingRepository
	prFilter       pullrequest.FilterFn
	uiPort         port.UIPort
}

// NewInitializeFirstCheckUseCase creates a new use case
func NewInitializeFirstCheckUseCase(
	prRepo pullrequest.PullRequestRepository,
	prTrackingRepo pullrequest.PRTrackingRepository,
	prFilter pullrequest.FilterFn,
	uiPort port.UIPort,
) *InitializeFirstCheckUseCase {
	return &InitializeFirstCheckUseCase{
		prRepo:         prRepo,
		prTrackingRepo: prTrackingRepo,
		prFilter:       prFilter,
		uiPort:         uiPort,
	}
}

// Execute runs the first-run initialization.
// Returns (true, allPRs, nil) on the first run ever, where allPRs contains all
// currently-open PRs that were marked as seen. The caller should use allPRs to
// seed any stateful use cases (e.g. pipeline-status tracking) so that the
// first regular check does not re-fire change events for every existing PR.
// Returns (false, nil, nil) when the tracking store is already populated.
func (uc *InitializeFirstCheckUseCase) Execute(ctx context.Context) (bool, []*pullrequest.PullRequest, error) {
	// Check if this is truly the first run ever
	isFirstRunEver := uc.prTrackingRepo.IsEmpty()

	if !isFirstRunEver {
		return false, nil, nil
	}

	log.Info().Msg("First run detected - marking all existing PRs as already seen")

	// Fetch all PRs
	requestedReviewPRs, err := uc.prRepo.FetchRequestedReviews()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching requested review PRs")
		return false, nil, err
	}

	userCreatedPRs, err := uc.prRepo.FetchUserCreated()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching user created PRs")
		return false, nil, err
	}

	// Filter draft PRs if configured
	requestedReviewPRs = uc.prFilter(requestedReviewPRs)
	userCreatedPRs = uc.prFilter(userCreatedPRs)

	// Mark all existing PRs as seen (no notifications, no asterisks on first run)
	for _, pr := range requestedReviewPRs {
		pr.MarkAsSeen()
	}
	for _, pr := range userCreatedPRs {
		pr.MarkAsSeen()
	}

	// Update the UI with seen state
	uc.uiPort.UpdateDisplay(requestedReviewPRs, userCreatedPRs, uc.prTrackingRepo)

	allPRs := append(requestedReviewPRs, userCreatedPRs...)
	log.Info().Msgf("First run complete: marked %d PRs as seen", len(allPRs))

	// Seed the tracking repo so the first regular check has a baseline for
	// closed/merged PR detection. On first run there is no previous enrichment
	// data, so we save the PRs as-is.
	if err := uc.prTrackingRepo.Save(allPRs); err != nil {
		log.Error().Err(err).Msg("InitializeFirstCheck: failed to seed tracking repo")
	}

	return true, allPRs, nil
}
