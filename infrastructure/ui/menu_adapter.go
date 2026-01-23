package ui

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"time"

	"github.com/getlantern/systray"
	"github.com/oak3/github-notifier/assets"
	"github.com/oak3/github-notifier/config"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// MenuAdapter adapts the systray menu to the MenuPort interface
type MenuAdapter struct {
	requestedPRsTitleMenuItem *systray.MenuItem
	userPRsTitleMenuItem      *systray.MenuItem
	requestedPRsMenuItems     []MenuItemPair
	userPRsMenuItems          []MenuItemPair
	quitMenuItem              *systray.MenuItem
	maxNumberOfRepos          int
	maxNumberOfPRs            int
	darkIcon                  []byte
	lightIcon                 []byte
	themeProvider             ThemeProvider
	ctx                       context.Context
	cancel                    context.CancelFunc
}

// ThemeProvider provides the current system theme
type ThemeProvider interface {
	GetSystemTheme() string
}

// MenuItemPair represents a parent menu item with its children
type MenuItemPair struct {
	Parent   *systray.MenuItem
	Children []*systray.MenuItem
}

// NewMenuAdapter creates a new menu adapter
func NewMenuAdapter(cfg *config.Config, themeProvider ThemeProvider) *MenuAdapter {
	ctx, cancel := context.WithCancel(context.Background())

	requestedPRsMenuItems := make([]MenuItemPair, cfg.MaxNumberOfRepos)
	for i := 0; i < cfg.MaxNumberOfRepos; i++ {
		requestedPRsMenuItems[i].Children = make([]*systray.MenuItem, cfg.MaxNumberOfPRs)
	}

	userPRsMenuItems := make([]MenuItemPair, cfg.MaxNumberOfRepos)
	for i := 0; i < cfg.MaxNumberOfRepos; i++ {
		userPRsMenuItems[i].Children = make([]*systray.MenuItem, cfg.MaxNumberOfPRs)
	}

	return &MenuAdapter{
		requestedPRsMenuItems: requestedPRsMenuItems,
		userPRsMenuItems:      userPRsMenuItems,
		maxNumberOfRepos:      cfg.MaxNumberOfRepos,
		maxNumberOfPRs:        cfg.MaxNumberOfPRs,
		themeProvider:         themeProvider,
		ctx:                   ctx,
		cancel:                cancel,
	}
}

// Setup initializes the menu
func (m *MenuAdapter) Setup() {
	// Use embedded icons
	m.SetThemeIcons(assets.DarkIcon, assets.LightIcon)
}

// SetThemeIcons sets the dark and light icons
func (m *MenuAdapter) SetThemeIcons(darkIcon, lightIcon []byte) {
	m.darkIcon = darkIcon
	m.lightIcon = lightIcon
	m.applyThemeIcon()
}

// applyThemeIcon applies the appropriate icon based on system theme
func (m *MenuAdapter) applyThemeIcon() {
	if m.darkIcon == nil || m.lightIcon == nil {
		return
	}

	theme := m.themeProvider.GetSystemTheme()
	if theme == "dark" {
		systray.SetIcon(m.darkIcon)
	} else {
		systray.SetIcon(m.lightIcon)
	}
}

// UpdateMenu updates the menu with pull requests
func (m *MenuAdapter) UpdateMenu(requestedReviewPRs, userCreatedPRs []*pullrequest.PullRequest) {
	m.requestedPRsTitleMenuItem = m.addOrUpdateParentMenuItem(
		m.requestedPRsTitleMenuItem,
		fmt.Sprintf("PRs Requested Reviews: %d", len(requestedReviewPRs)),
	)

	m.clearMenuItems(m.requestedPRsTitleMenuItem, m.requestedPRsMenuItems)

	if len(requestedReviewPRs) > 0 {
		m.buildPRSection(requestedReviewPRs, m.requestedPRsMenuItems)
	} else {
		m.requestedPRsMenuItems[0].Parent.SetTitle("(empty)   ")
		m.requestedPRsMenuItems[0].Parent.Show()
		m.requestedPRsMenuItems[0].Parent.Disable()
	}

	m.userPRsTitleMenuItem = m.addOrUpdateParentMenuItem(
		m.userPRsTitleMenuItem,
		fmt.Sprintf("Your PRs: %d", len(userCreatedPRs)),
	)

	m.clearMenuItems(m.userPRsTitleMenuItem, m.userPRsMenuItems)

	if len(userCreatedPRs) > 0 {
		m.buildPRSection(userCreatedPRs, m.userPRsMenuItems)
	} else {
		m.userPRsMenuItems[0].Parent.SetTitle("(empty)   ")
		m.userPRsMenuItems[0].Parent.Show()
		m.userPRsMenuItems[0].Parent.Disable()
	}

	totalPRs := len(requestedReviewPRs) + len(userCreatedPRs)
	systray.SetTooltip(fmt.Sprintf("GitHub Notifier: %d PRs", totalPRs))

	if m.quitMenuItem == nil {
		systray.AddSeparator()
		m.quitMenuItem = systray.AddMenuItem("Quit", "Quit the app")
		go m.handleQuitClick()
	}
}

// handleQuitClick handles the quit button click with proper cleanup
func (m *MenuAdapter) handleQuitClick() {
	select {
	case <-m.quitMenuItem.ClickedCh:
		systray.Quit()
	case <-m.ctx.Done():
		return
	}
}

// Shutdown cleans up the menu adapter
func (m *MenuAdapter) Shutdown() {
	m.cancel()
}

func (m *MenuAdapter) clearMenuItems(firstLevelTitleMenuItem *systray.MenuItem, firstLevelMenuItems []MenuItemPair) {
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

func (m *MenuAdapter) addOrUpdateParentMenuItem(menuItem *systray.MenuItem, title string) *systray.MenuItem {
	if menuItem == nil {
		menuItem = systray.AddMenuItem("", "")
	}
	menuItem.SetTitle(title + "   ")
	return menuItem
}

func (m *MenuAdapter) buildPRSection(prs []*pullrequest.PullRequest, parentMenuItem []MenuItemPair) {
	prsByRepo := m.groupPRsByRepository(prs)

	i := 0
	for repoName, repoPRs := range prsByRepo {
		parentMenuItem[i].Parent.SetTitle(repoName + "   ")

		for j, pr := range repoPRs {
			prTitle := m.formatPRTitle(pr)
			parentMenuItem[i].Children[j].SetTitle(prTitle)
			parentMenuItem[i].Children[j].Show()

			// Fix goroutine leak by using context
			go m.handlePRClick(parentMenuItem[i].Children[j], pr.URL())
		}

		parentMenuItem[i].Parent.Show()
		i++
	}
}

// handlePRClick handles PR menu item clicks with proper cleanup
func (m *MenuAdapter) handlePRClick(item *systray.MenuItem, url string) {
	for {
		select {
		case <-item.ClickedCh:
			m.openURL(url)
		case <-m.ctx.Done():
			return
		}
	}
}

// openURL opens a URL in the default browser
func (m *MenuAdapter) openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		log.Printf("Unsupported OS: %s", runtime.GOOS)
		return
	}
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to open URL: %v", err)
	}
}

// groupPRsByRepository groups PRs by their repository
func (m *MenuAdapter) groupPRsByRepository(prs []*pullrequest.PullRequest) map[string][]*pullrequest.PullRequest {
	grouped := make(map[string][]*pullrequest.PullRequest)
	for _, pr := range prs {
		grouped[pr.RepositoryName()] = append(grouped[pr.RepositoryName()], pr)
	}
	return grouped
}

// formatPRTitle returns a formatted PR title with age information
func (m *MenuAdapter) formatPRTitle(pr *pullrequest.PullRequest) string {
	return fmt.Sprintf("[%s] [#%d] %s", m.formatTimeAgo(pr.CreatedAt()), pr.Number(), pr.Title())
}

// formatTimeAgo returns a human-readable time difference
func (m *MenuAdapter) formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	hours := duration.Hours()
	days := hours / 24
	weeks := int(days / 7)

	if weeks > 0 {
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if days >= 2 {
		return fmt.Sprintf("%d days ago", int(days))
	} else {
		return fmt.Sprintf("%d hours ago", int(hours))
	}
}
