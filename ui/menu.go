package ui

import (
	"fmt"
	"log"
	"os"

	"github.com/getlantern/systray"
	"github.com/oak3/github-notifier/config"
	"github.com/oak3/github-notifier/domain"
)

// MenuManager handles systray menu operations
type MenuManager struct {
	requestedPRsTitleMenuItem *systray.MenuItem
	userPRsTitleMenuItem      *systray.MenuItem
	requestedPRsMenuItems     []MenuItemPair
	userPRsMenuItems          []MenuItemPair
	quitMenuItem              *systray.MenuItem
	onPRClick                 func(url string)
	maxNumberOfRepos          int
	maxNumberOfPRs            int
	darkIcon                  []byte
	lightIcon                 []byte
}

type MenuItemPair struct {
	Parent   *systray.MenuItem
	Children []*systray.MenuItem
}

// NewMenuManager creates a new menu manager
func NewMenuManager(cfg *config.Config, onPRClick func(url string)) *MenuManager {
	requestedPRsMenuItems := make([]MenuItemPair, cfg.MaxNumberOfRepos)
	for i := 0; i < cfg.MaxNumberOfRepos; i++ {
		requestedPRsMenuItems[i].Children = make([]*systray.MenuItem, cfg.MaxNumberOfPRs)
	}

	userPRsMenuItems := make([]MenuItemPair, cfg.MaxNumberOfRepos)
	for i := 0; i < cfg.MaxNumberOfRepos; i++ {
		userPRsMenuItems[i].Children = make([]*systray.MenuItem, cfg.MaxNumberOfPRs)
	}

	return &MenuManager{
		requestedPRsMenuItems: requestedPRsMenuItems,
		userPRsMenuItems:      userPRsMenuItems,
		onPRClick:             onPRClick,
		maxNumberOfRepos:      cfg.MaxNumberOfRepos,
		maxNumberOfPRs:        cfg.MaxNumberOfPRs,
	}
}

func (m *MenuManager) Setup() {
	darkIcon, err := os.ReadFile("icon.png")
	if err != nil {
		log.Printf("Failed to load icon: %v", err)
		darkIcon = []byte{} // fallback
	}
	lightIcon, err := os.ReadFile("icon_light.png")
	if err != nil {
		log.Printf("Failed to load icon_light: %v", err)
		lightIcon = []byte{} // fallback
	}
	m.SetThemeIcons(darkIcon, lightIcon)
}

// SetThemeIcons sets the dark and light icons for theme-aware display
func (m *MenuManager) SetThemeIcons(darkIcon, lightIcon []byte) {
	m.darkIcon = darkIcon
	m.lightIcon = lightIcon
	m.applyThemeIcon()
}

// applyThemeIcon applies the appropriate icon based on system theme
func (m *MenuManager) applyThemeIcon() {
	if m.darkIcon == nil || m.lightIcon == nil {
		return
	}

	theme := GetSystemTheme()
	if theme == "dark" {
		systray.SetIcon(m.darkIcon)
	} else {
		systray.SetIcon(m.lightIcon)
	}
}

// BuildMenu updates the systray menu with PR data
func (m *MenuManager) BuildMenu(requestedReviewPRs []domain.PullRequest, usersPRs []domain.PullRequest) {
	m.requestedPRsTitleMenuItem = m.addOrUpdateParentMenuItem(m.requestedPRsTitleMenuItem, fmt.Sprintf("PRs Requested Reviews: %d", len(requestedReviewPRs)))

	m.clearMenuItems(m.requestedPRsTitleMenuItem, m.requestedPRsMenuItems)

	// Build requested reviews section
	if len(requestedReviewPRs) > 0 {
		m.buildPRSection(requestedReviewPRs, m.requestedPRsMenuItems)
	} else {
		m.requestedPRsMenuItems[0].Parent.SetTitle("(empty)")
		m.requestedPRsMenuItems[0].Parent.Show()
		m.requestedPRsMenuItems[0].Parent.Disable()
	}

	m.userPRsTitleMenuItem = m.addOrUpdateParentMenuItem(m.userPRsTitleMenuItem, fmt.Sprintf("Your PRs: %d", len(usersPRs)))

	// Reset user's PRs menu items
	m.clearMenuItems(m.userPRsTitleMenuItem, m.userPRsMenuItems)

	// Build user's PRs section
	if len(usersPRs) > 0 {
		m.buildPRSection(usersPRs, m.userPRsMenuItems)
	} else {
		m.userPRsMenuItems[0].Parent.SetTitle("(empty)")
		m.userPRsMenuItems[0].Parent.Show()
		m.userPRsMenuItems[0].Parent.Disable()
	}

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

func (m *MenuManager) clearMenuItems(firstLevelTitleMenuItem *systray.MenuItem, firstLevelMenuItems []MenuItemPair) {
	for i := 0; i < m.maxNumberOfRepos; i++ {
		if firstLevelMenuItems[i].Parent == nil {
			firstLevelMenuItems[i].Parent = firstLevelTitleMenuItem.AddSubMenuItem("", "")
		}
		firstLevelMenuItems[i].Parent.Enable()
		firstLevelMenuItems[i].Parent.Hide()
		for j := 0; j < m.maxNumberOfPRs; j++ {
			if firstLevelMenuItems[i].Children[j] == nil {
				firstLevelMenuItems[i].Children[j] = firstLevelMenuItems[i].Parent.AddSubMenuItem("", "")
			}
			firstLevelMenuItems[i].Children[j].Enable()
			firstLevelMenuItems[i].Children[j].Hide()
		}
	}
}

func (m *MenuManager) addOrUpdateParentMenuItem(menuItem *systray.MenuItem, title string) *systray.MenuItem {
	if menuItem == nil {
		menuItem = systray.AddMenuItem("", "")
	}
	menuItem.SetTitle(title)
	return menuItem
}

func (m *MenuManager) buildPRSection(prs []domain.PullRequest, parentMenuItem []MenuItemPair) {
	prsByRepo := groupPRsByRepository(prs)

	i := 0
	for repoName, repoPRs := range prsByRepo {
		parentMenuItem[i].Parent.SetTitle(repoName)

		for j, pr := range repoPRs {
			prTitle := formatPRTitle(pr)

			parentMenuItem[i].Children[j].SetTitle(prTitle)
			parentMenuItem[i].Children[j].Show()
			child := parentMenuItem[i].Children[j]
			prURL := pr.URL

			go func(item *systray.MenuItem, url string) {
				for range item.ClickedCh {
					m.onPRClick(url)
				}
			}(child, prURL)
		}

		parentMenuItem[i].Parent.Show()
		i++
	}
}
