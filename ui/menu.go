package ui

import (
	"fmt"

	"github.com/getlantern/systray"
	"github.com/oak3/github-notifier/domain"
)

// MenuManager handles systray menu operations
type MenuManager struct {
	requestedPRsTitleMenuItem *systray.MenuItem
	userPRsTitleMenuItem      *systray.MenuItem
	quitMenuItem              *systray.MenuItem
	repoMenuItems             map[string]*systray.MenuItem
	prMenuItems               map[string]*systray.MenuItem
	onPRClick                 func(url string)
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

	if m.requestedPRsTitleMenuItem == nil {
		m.requestedPRsTitleMenuItem = systray.AddMenuItem(fmt.Sprintf("PRs Requested Reviews: %d", len(requestedReviewPRs)), "")
	} else {
		m.requestedPRsTitleMenuItem.SetTitle(fmt.Sprintf("PRs Requested Reviews: %d", len(requestedReviewPRs)))
	}

	// Build requested reviews section
	if len(requestedReviewPRs) > 0 {
		m.buildPRSection(requestedReviewPRs, "review:", currentPRItems, currentRepoItems, m.requestedPRsTitleMenuItem)
	}

	if m.userPRsTitleMenuItem == nil {
		systray.AddSeparator()
		m.userPRsTitleMenuItem = systray.AddMenuItem(fmt.Sprintf("Your PRs: %d", len(usersPRs)), "")
	} else {
		m.userPRsTitleMenuItem.SetTitle(fmt.Sprintf("Your PRs: %d", len(usersPRs)))
	}

	// Build user's PRs section
	if len(usersPRs) > 0 {
		m.buildPRSection(usersPRs, "user:", currentPRItems, currentRepoItems, m.userPRsTitleMenuItem)
	}

	// Clean up old items
	m.hideRemovedItems(currentPRItems, currentRepoItems)

	// Update tooltip
	totalPRs := len(requestedReviewPRs) + len(usersPRs)
	systray.SetTooltip(fmt.Sprintf("GitHub Notifier: %d PRs", totalPRs))

	// Add quit button
	if m.quitMenuItem == nil {
		systray.AddSeparator()
		m.quitMenuItem = systray.AddMenuItem("Quit", "Quit the app")
		go func() {
			for range m.quitMenuItem.ClickedCh {
				systray.Quit()
			}
		}()
	}
}

func (m *MenuManager) buildPRSection(prs []domain.PullRequest, prefix string, currentPRItems, currentRepoItems map[string]*systray.MenuItem, parentMenuItem *systray.MenuItem) {
	prsByRepo := groupPRsByRepository(prs)

	for repoName, repoPRs := range prsByRepo {
		repoKey := prefix + repoName
		var repoItem *systray.MenuItem

		if item, ok := m.repoMenuItems[repoKey]; ok {
			repoItem = item
		} else {
			repoItem = parentMenuItem.AddSubMenuItem(repoName, "")
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
