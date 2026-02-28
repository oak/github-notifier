package ui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/rs/zerolog/log"

	"github.com/oak3/github-notifier/assets"
	"github.com/oak3/github-notifier/domain/pullrequest"
)

// MenuAdapter adapts the systray menu to the MenuPort interface
type MenuAdapter struct {
	authenticatedUser         string            // GitHub login of the authenticated user
	userMenuItem              *systray.MenuItem // Disabled menu item showing the authenticated user
	requestedPRsTitleMenuItem *systray.MenuItem
	userPRsTitleMenuItem      *systray.MenuItem
	requestedPRsMenuItems     []MenuItemPair
	userPRsMenuItems          []MenuItemPair
	quitMenuItem              *systray.MenuItem
	waitingMenuItems          []*systray.MenuItem // Menu items shown during waiting-for-config state
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
func NewMenuAdapter(maxNumberOfRepos, maxNumberOfPRs int, themeProvider ThemeProvider, authenticatedUser string) *MenuAdapter {
	ctx, cancel := context.WithCancel(context.Background())

	requestedPRsMenuItems := make([]MenuItemPair, maxNumberOfRepos)
	for i := 0; i < maxNumberOfRepos; i++ {
		requestedPRsMenuItems[i].Children = make([]*systray.MenuItem, maxNumberOfPRs)
	}

	userPRsMenuItems := make([]MenuItemPair, maxNumberOfRepos)
	for i := 0; i < maxNumberOfRepos; i++ {
		userPRsMenuItems[i].Children = make([]*systray.MenuItem, maxNumberOfPRs)
	}

	return &MenuAdapter{
		authenticatedUser:     authenticatedUser,
		requestedPRsMenuItems: requestedPRsMenuItems,
		userPRsMenuItems:      userPRsMenuItems,
		maxNumberOfRepos:      maxNumberOfRepos,
		maxNumberOfPRs:        maxNumberOfPRs,
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

// SetupWaitingState shows a minimal systray menu indicating the app is waiting
// for a valid configuration. It displays the config file path and a quit button.
func (m *MenuAdapter) SetupWaitingState(configFilePath string) {
	m.SetThemeIcons(assets.DarkIcon, assets.LightIcon)
	systray.SetTooltip("GitHub Notifier — waiting for configuration")

	waitingItem := systray.AddMenuItem("⚠ Waiting for GitHub token...", "Set your GitHub token in the config file")
	waitingItem.Disable()

	configItem := systray.AddMenuItem("Config: "+configFilePath, "Config file location")
	configItem.Disable()

	// Track these items so ClearWaitingState can hide them
	m.waitingMenuItems = []*systray.MenuItem{waitingItem, configItem}

	systray.AddSeparator()
	m.quitMenuItem = systray.AddMenuItem("Quit", "Quit the app")
	go m.handleQuitClick()
}

// ClearWaitingState hides the waiting-mode menu items so the normal
// PR menu can take over. Called when a valid config is detected.
func (m *MenuAdapter) ClearWaitingState() {
	for _, item := range m.waitingMenuItems {
		item.Hide()
	}
	m.waitingMenuItems = nil

	// Hide the old quit item — a new one will be created by UpdateDisplay
	if m.quitMenuItem != nil {
		m.quitMenuItem.Hide()
		m.quitMenuItem = nil
	}
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

	// Show authenticated user as first inactive menu item
	if m.authenticatedUser != "" {
		if m.userMenuItem == nil {
			m.userMenuItem = systray.AddMenuItem("@"+m.authenticatedUser+"   ", "Signed in as "+m.authenticatedUser)
			m.userMenuItem.Disable()
			systray.AddSeparator()
		}
	}

	// Add asterisk to section title if it contains unseen PRs
	requestedReviewTitle := fmt.Sprintf("PRs Requested Reviews: %d", len(requestedReviewPRs))
	if m.hasUnseenPRs(requestedReviewPRs) {
		requestedReviewTitle = "* " + requestedReviewTitle
	}
	m.requestedPRsTitleMenuItem = m.addOrUpdateParentMenuItem(
		m.requestedPRsTitleMenuItem,
		requestedReviewTitle,
	)

	m.updatePRSection(requestedReviewPRs, m.requestedPRsMenuItems, m.requestedPRsTitleMenuItem)

	// Add asterisk to section title if it contains unseen PRs
	userPRsTitle := fmt.Sprintf("Your PRs: %d", len(userCreatedPRs))
	if m.hasUnseenPRs(userCreatedPRs) {
		userPRsTitle = "* " + userPRsTitle
	}
	m.userPRsTitleMenuItem = m.addOrUpdateParentMenuItem(
		m.userPRsTitleMenuItem,
		userPRsTitle,
	)

	m.updatePRSection(userCreatedPRs, m.userPRsMenuItems, m.userPRsTitleMenuItem)

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

func (m *MenuAdapter) addOrUpdateParentMenuItem(menuItem *systray.MenuItem, title string) *systray.MenuItem {
	if menuItem == nil {
		menuItem = systray.AddMenuItem("", "")
	}
	menuItem.SetTitle(title + "   ")
	return menuItem
}

// updatePRSection updates the PR menu section in-place without first clearing all items,
// which avoids the menu briefly disappearing between a clear and rebuild.
func (m *MenuAdapter) updatePRSection(prs []*pullrequest.PullRequest, menuItems []MenuItemPair, sectionTitle *systray.MenuItem) {
	if len(prs) == 0 {
		// Show "(empty)" placeholder in the first slot
		if menuItems[0].Parent == nil {
			menuItems[0].Parent = sectionTitle.AddSubMenuItem("", "")
		}
		menuItems[0].Parent.SetTitle("(empty)   ")
		menuItems[0].Parent.Disable()
		menuItems[0].Parent.Show()

		// Hide all child items of slot 0
		for j := 0; j < m.maxNumberOfPRs; j++ {
			if menuItems[0].Children[j] != nil {
				menuItems[0].Children[j].Hide()
			}
		}

		// Hide remaining repo slots
		for i := 1; i < m.maxNumberOfRepos; i++ {
			if menuItems[i].Parent != nil {
				menuItems[i].Parent.Hide()
			}
		}
		return
	}

	prsByRepo := m.groupPRsByRepository(prs)

	// Sort repository names to ensure consistent ordering
	repoNames := make([]string, 0, len(prsByRepo))
	for repoName := range prsByRepo {
		repoNames = append(repoNames, repoName)
	}
	sort.Strings(repoNames)

	// Update repo slots that are in use
	for i, repoName := range repoNames {
		repoPRs := prsByRepo[repoName]

		if menuItems[i].Parent == nil {
			menuItems[i].Parent = sectionTitle.AddSubMenuItem("", "")
		}

		// Add asterisk to repository name if it contains unseen PRs
		repoTitle := repoName
		if m.hasUnseenPRs(repoPRs) {
			repoTitle = "* " + repoName
		}
		menuItems[i].Parent.SetTitle(repoTitle + "   ")
		menuItems[i].Parent.Enable()

		// Update PR child slots that are in use
		for j, pr := range repoPRs {
			if menuItems[i].Children[j] == nil {
				menuItems[i].Children[j] = menuItems[i].Parent.AddSubMenuItem("", "")
			}

			menuItem := menuItems[i].Children[j]
			menuItem.SetTitle(m.formatPRTitle(pr))
			menuItem.Enable()
			menuItem.Show()

			// Cancel old goroutine for this menu item and start a new one
			m.menuItemCancelsMu.Lock()
			if cancelFunc, exists := m.menuItemCancels[menuItem]; exists {
				cancelFunc()
			}
			itemCtx, itemCancel := context.WithCancel(m.ctx)
			m.menuItemCancels[menuItem] = itemCancel
			m.menuItemCancelsMu.Unlock()

			go m.handlePRClick(itemCtx, menuItem, pr)
		}

		// Hide child slots no longer in use for this repo
		for j := len(repoPRs); j < m.maxNumberOfPRs; j++ {
			if menuItems[i].Children[j] != nil {
				menuItems[i].Children[j].Hide()
			}
		}

		menuItems[i].Parent.Show()
	}

	// Hide repo slots no longer in use
	for i := len(repoNames); i < m.maxNumberOfRepos; i++ {
		if menuItems[i].Parent != nil {
			menuItems[i].Parent.Hide()
		}
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
		log.Warn().Msgf("Unsupported OS: %s", runtime.GOOS)
		return
	}
	if err := cmd.Run(); err != nil {
		log.Error().Msgf("Failed to open URL: %v", err)
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

// formatPRTitle returns a formatted PR title with age information, unseen indicator, review states, and pipeline status
func (m *MenuAdapter) formatPRTitle(pr *pullrequest.PullRequest) string {
	prefix := ""
	m.clickedPRsMu.RLock()
	clicked := m.clickedPRs[pr.URL()]
	m.clickedPRsMu.RUnlock()
	if !clicked {
		prefix = "* "
	}

	reviewSuffix := ""
	summary := pr.ReviewSummary()
	if !summary.IsEmpty() {
		reviewSuffix = " " + summary.FormatForMenu()
	}

	pipelinePrefix := ""
	if pr.PipelineStatus() != pullrequest.PipelineStatusUnknown {
		pipelinePrefix = pr.PipelineStatus().Emoji() + " "
	}

	return fmt.Sprintf("%s[%s] [#%d] %s%s", prefix, m.formatTimeAgo(pr.CreatedAt()), pr.Number(), pipelinePrefix+pr.Title(), reviewSuffix)
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
	}
	if days >= 2 {
		return fmt.Sprintf("%d days ago", int(days))
	}
	return fmt.Sprintf("%d hours ago", int(hours))
}
