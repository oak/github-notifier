package ui

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sort"
	"sync"
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
	trackingService           *pullrequest.TrackingService
	requestedReviewPRs        []*pullrequest.PullRequest
	userCreatedPRs            []*pullrequest.PullRequest
	clickedPRs                map[string]bool                          // Track which PRs have been clicked in the menu
	clickedPRsMu              sync.RWMutex                             // Protects clickedPRs from concurrent access
	menuItemCancels           map[*systray.MenuItem]context.CancelFunc // Track cancel funcs for each menu item
	menuItemCancelsMu         sync.RWMutex                             // Protects menuItemCancels
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
		clickedPRs:            make(map[string]bool),
		menuItemCancels:       make(map[*systray.MenuItem]context.CancelFunc),
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

// UpdateDisplay implements the UIPort interface for systray menu display
// This adapter specifically renders PRs as a system tray menu with dropdowns
func (m *MenuAdapter) UpdateDisplay(requestedReviewPRs, userCreatedPRs []*pullrequest.PullRequest, trackingService *pullrequest.TrackingService) {
	// Store tracking service and PR lists for use in menu item formatting
	m.trackingService = trackingService
	m.requestedReviewPRs = requestedReviewPRs
	m.userCreatedPRs = userCreatedPRs

	// Sync clickedPRs with tracking service's seen state (bidirectional sync)
	// If a PR has been marked as seen, consider it clicked in the menu
	// If a PR has been marked as unseen (e.g., new activity), remove it from clicked
	// This way: in-memory repos won't show asterisks on first load
	// And persistent repos will preserve the clicked state across restarts
	// And PRs with new activity will show asterisks again
	m.clickedPRsMu.Lock()
	for _, pr := range requestedReviewPRs {
		if trackingService.HasBeenSeen(pr.Identifier()) {
			if !m.clickedPRs[pr.URL()] {
				m.clickedPRs[pr.URL()] = true
			}
		} else {
			// PR is unseen - remove from clickedPRs to show asterisk
			delete(m.clickedPRs, pr.URL())
		}
	}
	for _, pr := range userCreatedPRs {
		if trackingService.HasBeenSeen(pr.Identifier()) {
			if !m.clickedPRs[pr.URL()] {
				m.clickedPRs[pr.URL()] = true
			}
		} else {
			// PR is unseen - remove from clickedPRs to show asterisk
			delete(m.clickedPRs, pr.URL())
		}
	}
	m.clickedPRsMu.Unlock()

	// Add asterisk to section title if it contains unseen PRs
	requestedReviewTitle := fmt.Sprintf("PRs Requested Reviews: %d", len(requestedReviewPRs))
	if m.hasUnseenPRs(requestedReviewPRs) {
		requestedReviewTitle = "* " + requestedReviewTitle
	}
	m.requestedPRsTitleMenuItem = m.addOrUpdateParentMenuItem(
		m.requestedPRsTitleMenuItem,
		requestedReviewTitle,
	)

	m.clearMenuItems(m.requestedPRsTitleMenuItem, m.requestedPRsMenuItems)

	if len(requestedReviewPRs) > 0 {
		m.buildPRSection(requestedReviewPRs, m.requestedPRsMenuItems)
	} else {
		m.requestedPRsMenuItems[0].Parent.SetTitle("(empty)   ")
		m.requestedPRsMenuItems[0].Parent.Show()
		m.requestedPRsMenuItems[0].Parent.Disable()
	}

	// Add asterisk to section title if it contains unseen PRs
	userPRsTitle := fmt.Sprintf("Your PRs: %d", len(userCreatedPRs))
	if m.hasUnseenPRs(userCreatedPRs) {
		userPRsTitle = "* " + userPRsTitle
	}
	m.userPRsTitleMenuItem = m.addOrUpdateParentMenuItem(
		m.userPRsTitleMenuItem,
		userPRsTitle,
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

	// Sort repository names to ensure consistent ordering
	repoNames := make([]string, 0, len(prsByRepo))
	for repoName := range prsByRepo {
		repoNames = append(repoNames, repoName)
	}
	sort.Strings(repoNames)

	i := 0
	for _, repoName := range repoNames {
		repoPRs := prsByRepo[repoName]
		// Add asterisk to repository name if it contains unseen PRs
		repoTitle := repoName
		if m.hasUnseenPRs(repoPRs) {
			repoTitle = "* " + repoName
		}
		parentMenuItem[i].Parent.SetTitle(repoTitle + "   ")

		for j, pr := range repoPRs {
			pr := pr // Capture loop variable to avoid closure bug
			menuItem := parentMenuItem[i].Children[j]

			prTitle := m.formatPRTitle(pr)
			menuItem.SetTitle(prTitle)
			menuItem.Show()

			// Cancel old goroutine for this menu item if it exists
			m.menuItemCancelsMu.Lock()
			if cancelFunc, exists := m.menuItemCancels[menuItem]; exists {
				cancelFunc() // Cancel the old goroutine
			}

			// Create new context for this menu item
			itemCtx, itemCancel := context.WithCancel(m.ctx)
			m.menuItemCancels[menuItem] = itemCancel
			m.menuItemCancelsMu.Unlock()

			// Start new goroutine with its own context
			go m.handlePRClick(itemCtx, menuItem, pr)
		}

		parentMenuItem[i].Parent.Show()
		i++
	}
}

// handlePRClick handles PR menu item clicks with proper cleanup
func (m *MenuAdapter) handlePRClick(ctx context.Context, item *systray.MenuItem, pr *pullrequest.PullRequest) {
	for {
		select {
		case <-item.ClickedCh:
			// Mark as clicked in our local tracking
			m.clickedPRsMu.Lock()
			m.clickedPRs[pr.URL()] = true
			m.clickedPRsMu.Unlock()
			// Remove asterisk from title immediately
			newTitle := m.formatPRTitle(pr)
			item.SetTitle(newTitle)
			// Refresh the entire menu hierarchy to update asterisks
			m.refreshMenuHierarchy()
			// Open the URL
			m.openURL(pr.URL())
		case <-ctx.Done():
			return
		}
	}
}

// refreshMenuHierarchy updates the asterisks in the menu hierarchy after a PR is marked as seen
func (m *MenuAdapter) refreshMenuHierarchy() {
	// Update requested review section title
	if m.requestedPRsTitleMenuItem != nil && m.requestedReviewPRs != nil {
		requestedReviewTitle := fmt.Sprintf("PRs Requested Reviews: %d", len(m.requestedReviewPRs))
		if m.hasUnseenPRs(m.requestedReviewPRs) {
			requestedReviewTitle = "* " + requestedReviewTitle
		}
		m.requestedPRsTitleMenuItem.SetTitle(requestedReviewTitle + "   ")
	}

	// Update user PRs section title
	if m.userPRsTitleMenuItem != nil && m.userCreatedPRs != nil {
		userPRsTitle := fmt.Sprintf("Your PRs: %d", len(m.userCreatedPRs))
		if m.hasUnseenPRs(m.userCreatedPRs) {
			userPRsTitle = "* " + userPRsTitle
		}
		m.userPRsTitleMenuItem.SetTitle(userPRsTitle + "   ")
	}

	// Update repository menu items in requested reviews section
	if m.requestedReviewPRs != nil {
		m.refreshRepositoryTitles(m.requestedReviewPRs, m.requestedPRsMenuItems)
	}

	// Update repository menu items in user PRs section
	if m.userCreatedPRs != nil {
		m.refreshRepositoryTitles(m.userCreatedPRs, m.userPRsMenuItems)
	}
}

// refreshRepositoryTitles updates the asterisks on repository menu items
func (m *MenuAdapter) refreshRepositoryTitles(prs []*pullrequest.PullRequest, menuItems []MenuItemPair) {
	prsByRepo := m.groupPRsByRepository(prs)

	// Sort repository names to ensure consistent ordering
	repoNames := make([]string, 0, len(prsByRepo))
	for repoName := range prsByRepo {
		repoNames = append(repoNames, repoName)
	}
	sort.Strings(repoNames)

	i := 0
	for _, repoName := range repoNames {
		repoPRs := prsByRepo[repoName]
		if i >= len(menuItems) || menuItems[i].Parent == nil {
			break
		}
		repoTitle := repoName
		if m.hasUnseenPRs(repoPRs) {
			repoTitle = "* " + repoName
		}
		menuItems[i].Parent.SetTitle(repoTitle + "   ")
		i++
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

// formatPRTitle returns a formatted PR title with age information and unseen indicator
func (m *MenuAdapter) formatPRTitle(pr *pullrequest.PullRequest) string {
	prefix := ""
	m.clickedPRsMu.RLock()
	clicked := m.clickedPRs[pr.URL()]
	m.clickedPRsMu.RUnlock()
	if !clicked {
		prefix = "* "
	}
	return fmt.Sprintf("%s[%s] [#%d] %s", prefix, m.formatTimeAgo(pr.CreatedAt()), pr.Number(), pr.Title())
}

// hasUnseenPRs checks if any PRs in the list have not been clicked
func (m *MenuAdapter) hasUnseenPRs(prs []*pullrequest.PullRequest) bool {
	m.clickedPRsMu.RLock()
	defer m.clickedPRsMu.RUnlock()
	for _, pr := range prs {
		if !m.clickedPRs[pr.URL()] {
			return true
		}
	}
	return false
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
