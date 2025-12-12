package ui

import (
	"fmt"

	"github.com/getlantern/systray"
	"github.com/oak3/github-notifier/domain"
)

// MenuManager handles systray menu operations
type MenuManager struct {
	prMenuItems   map[string]*systray.MenuItem
	repoMenuItems map[string]*systray.MenuItem
	onPRClick     func(url string)
}

// NewMenuManager creates a new menu manager
func NewMenuManager(onPRClick func(url string)) *MenuManager {
	return &MenuManager{
		prMenuItems:   make(map[string]*systray.MenuItem),
		repoMenuItems: make(map[string]*systray.MenuItem),
		onPRClick:     onPRClick,
	}
}

// BuildMenu updates the systray menu with PR data
func (m *MenuManager) BuildMenu(requestedReviewPRs []domain.PullRequest, usersPRs []domain.PullRequest) {
	currentPRItems := make(map[string]*systray.MenuItem)
	currentRepoItems := make(map[string]*systray.MenuItem)

	// Build requested reviews section
	if len(requestedReviewPRs) > 0 {
		systray.AddMenuItem(fmt.Sprintf("PRs Requested Reviews: %d", len(requestedReviewPRs)), "").Disable()
		m.buildPRSection(requestedReviewPRs, "review:", currentPRItems, currentRepoItems)
		systray.AddSeparator()
	}

	// Build user's PRs section
	if len(usersPRs) > 0 {
		systray.AddMenuItem(fmt.Sprintf("Your PRs: %d", len(usersPRs)), "").Disable()
		m.buildPRSection(usersPRs, "user:", currentPRItems, currentRepoItems)
		systray.AddSeparator()
	}

	// Clean up old items
	m.hideRemovedItems(currentPRItems, currentRepoItems)

	// Update tooltip
	totalPRs := len(requestedReviewPRs) + len(usersPRs)
	systray.SetTooltip(fmt.Sprintf("GitHub Notifier: %d PRs", totalPRs))

	// Add quit button
	mQuit := systray.AddMenuItem("Quit", "Quit the app")
	go func() {
		for range mQuit.ClickedCh {
			systray.Quit()
		}
	}()
}

func (m *MenuManager) buildPRSection(prs []domain.PullRequest, prefix string, currentPRItems, currentRepoItems map[string]*systray.MenuItem) {
	prsByRepo := groupPRsByRepository(prs)

	for repoName, repoPRs := range prsByRepo {
		repoKey := prefix + repoName
		var repoItem *systray.MenuItem

		if item, ok := m.repoMenuItems[repoKey]; ok {
			repoItem = item
		} else {
			repoItem = systray.AddMenuItem(repoName, "")
		}

		currentRepoItems[repoKey] = repoItem

		for _, pr := range repoPRs {
			prKey := pr.URL
			prTitle := formatPRTitle(pr)
			var prItem *systray.MenuItem

			if item, ok := m.prMenuItems[prKey]; ok {
				prItem = item
				prItem.SetTitle(prTitle)
				prItem.Show()
			} else {
				prItem = repoItem.AddSubMenuItem(prTitle, "")
				url := pr.URL
				go func() {
					for range prItem.ClickedCh {
						m.onPRClick(url)
					}
				}()
			}

			currentPRItems[prKey] = prItem
		}
	}
}

func (m *MenuManager) hideRemovedItems(currentPRItems, currentRepoItems map[string]*systray.MenuItem) {
	for url, item := range m.prMenuItems {
		if _, ok := currentPRItems[url]; !ok {
			item.Hide()
		}
	}
	m.prMenuItems = currentPRItems

	for repoKey, item := range m.repoMenuItems {
		if _, ok := currentRepoItems[repoKey]; !ok {
			item.Hide()
		}
	}
	m.repoMenuItems = currentRepoItems
}
