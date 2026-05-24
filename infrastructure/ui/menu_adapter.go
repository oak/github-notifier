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

	"github.com/oak3/github-notifier/application/port"
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
	requestedPRsEmptyItem     *systray.MenuItem   // Separate (empty) placeholder for requested reviews section
	userPRsEmptyItem          *systray.MenuItem   // Separate (empty) placeholder for user PRs section
	waitingMenuItems          []*systray.MenuItem // Menu items shown during waiting-for-config state
	ignoreHandler             IgnoreMenuHandler   // registered before menu is built; called when Ignore Rules is clicked
	maxNumberOfRepos          int
	maxNumberOfPRs            int
	darkIcon                  []byte
	lightIcon                 []byte
	themeProvider             ThemeProvider
	requestedReviewPRs        []*pullrequest.PullRequest
	userCreatedPRs            []*pullrequest.PullRequest
	clickedPRs                map[string]bool                          // Track which PRs have been clicked in the menu
	clickedPRsMu              sync.RWMutex                             // Protects clickedPRs from concurrent access
	menuItemCancels           map[*systray.MenuItem]context.CancelFunc // Track cancel funcs for each menu item
	menuItemCancelsMu         sync.RWMutex                             // Protects menuItemCancels
	ctx                       context.Context
	cancel                    context.CancelFunc
	menuInitialized           bool
}

// IgnoreMenuHandler is a callback for when the Ignore Rules menu item is clicked.
type IgnoreMenuHandler func()

// RegisterIgnoreHandler stores the callback to invoke when "Configuration > Ignore Rules"
// is clicked. It must be called before the first UpdateDisplay so that
// initializeMenuStructure can wire up the menu item at the right position.
func (m *MenuAdapter) RegisterIgnoreHandler(handler IgnoreMenuHandler) {
	m.ignoreHandler = handler
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

// initializeMenuStructure pre-creates all systray menu items on the first
// UpdateDisplay call.  On Linux, dbusmenu/appindicator stacks do not reliably
// display items added after the initial menu layout, so every possible slot
// (section titles, "(empty)" placeholders, repo parents, and PR children)
// must exist in the widget tree from the start.  After initialisation only
// Show / Hide / SetTitle are used — never another AddSubMenuItem.
func (m *MenuAdapter) initializeMenuStructure() {
	if m.menuInitialized {
		return
	}
	m.menuInitialized = true

	// Authenticated user label
	if m.authenticatedUser != "" {
		m.userMenuItem = systray.AddMenuItem("@"+m.authenticatedUser+"   ", "Signed in as "+m.authenticatedUser)
		m.userMenuItem.Disable()
		systray.AddSeparator()
	}

	// ── Requested-reviews section ──
	m.requestedPRsTitleMenuItem = systray.AddMenuItem("PRs Requested Reviews: 0   ", "")
	m.requestedPRsEmptyItem = m.requestedPRsTitleMenuItem.AddSubMenuItem("(empty)   ", "")
	m.requestedPRsEmptyItem.Disable()
	for i := 0; i < m.maxNumberOfRepos; i++ {
		m.requestedPRsMenuItems[i].Parent = m.requestedPRsTitleMenuItem.AddSubMenuItem("", "")
		m.requestedPRsMenuItems[i].Parent.Hide()
		for j := 0; j < m.maxNumberOfPRs; j++ {
			m.requestedPRsMenuItems[i].Children[j] = m.requestedPRsMenuItems[i].Parent.AddSubMenuItem("", "")
			m.requestedPRsMenuItems[i].Children[j].Hide()
		}
	}

	// ── User PRs section ──
	m.userPRsTitleMenuItem = systray.AddMenuItem("Your PRs: 0   ", "")
	m.userPRsEmptyItem = m.userPRsTitleMenuItem.AddSubMenuItem("(empty)   ", "")
	m.userPRsEmptyItem.Disable()
	for i := 0; i < m.maxNumberOfRepos; i++ {
		m.userPRsMenuItems[i].Parent = m.userPRsTitleMenuItem.AddSubMenuItem("", "")
		m.userPRsMenuItems[i].Parent.Hide()
		for j := 0; j < m.maxNumberOfPRs; j++ {
			m.userPRsMenuItems[i].Children[j] = m.userPRsMenuItems[i].Parent.AddSubMenuItem("", "")
			m.userPRsMenuItems[i].Children[j].Hide()
		}
	}

	// ── Configuration ──
	if m.ignoreHandler != nil {
		systray.AddSeparator()
		configParent := systray.AddMenuItem("Configuration", "Configuration options")
		ignoreItem := configParent.AddSubMenuItem("Ignore Rules", "Edit ignore.yaml rules")
		handler := m.ignoreHandler
		go func() {
			for {
				select {
				case <-ignoreItem.ClickedCh:
					handler()
				case <-m.ctx.Done():
					return
				}
			}
		}()
	}

	// ── Quit ──
	systray.AddSeparator()
	m.quitMenuItem = systray.AddMenuItem("Quit", "Quit the app")
	go m.handleQuitClick()
}

// UpdateDisplay implements the UIPort interface for systray menu display
// This adapter specifically renders PRs as a system tray menu with dropdowns
func (m *MenuAdapter) UpdateDisplay(requestedReviewPRs, userCreatedPRs []*pullrequest.PullRequest, seenReader port.PullRequestSeenReader) {
	// Pre-create all menu item slots on the first call so the
	// dbusmenu/appindicator model contains every node from the start.
	m.initializeMenuStructure()

	// Store PR lists for use in menu item formatting
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
		if seenReader.HasBeenSeen(pr.Identifier()) {
			if !m.clickedPRs[pr.URL()] {
				m.clickedPRs[pr.URL()] = true
			}
		} else {
			// PR is unseen - remove from clickedPRs to show asterisk
			delete(m.clickedPRs, pr.URL())
		}
	}
	for _, pr := range userCreatedPRs {
		if seenReader.HasBeenSeen(pr.Identifier()) {
			if !m.clickedPRs[pr.URL()] {
				m.clickedPRs[pr.URL()] = true
			}
		} else {
			// PR is unseen - remove from clickedPRs to show asterisk
			delete(m.clickedPRs, pr.URL())
		}
	}
	m.clickedPRsMu.Unlock()

	// Update section titles (already created by initializeMenuStructure)
	requestedReviewTitle := fmt.Sprintf("PRs Requested Reviews: %d", len(requestedReviewPRs))
	if m.hasUnseenPRs(requestedReviewPRs) {
		requestedReviewTitle = "* " + requestedReviewTitle
	}
	m.requestedPRsTitleMenuItem.SetTitle(requestedReviewTitle + "   ")

	m.updatePRSection(requestedReviewPRs, m.requestedPRsMenuItems, &m.requestedPRsEmptyItem)

	userPRsTitle := fmt.Sprintf("Your PRs: %d", len(userCreatedPRs))
	if m.hasUnseenPRs(userCreatedPRs) {
		userPRsTitle = "* " + userPRsTitle
	}
	m.userPRsTitleMenuItem.SetTitle(userPRsTitle + "   ")

	m.updatePRSection(userCreatedPRs, m.userPRsMenuItems, &m.userPRsEmptyItem)

	totalPRs := len(requestedReviewPRs) + len(userCreatedPRs)
	systray.SetTooltip(fmt.Sprintf("GitHub Notifier: %d PRs", totalPRs))
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

// updatePRSection updates a PR menu section (requested-reviews or user-PRs)
// in-place using pre-created menu item slots.  No new items are ever created
// here — initializeMenuStructure has already allocated every slot.
func (m *MenuAdapter) updatePRSection(prs []*pullrequest.PullRequest, menuItems []MenuItemPair, emptyItem **systray.MenuItem) {
	if len(prs) == 0 {
		// Show the pre-created "(empty)" placeholder.
		// SetTitle forces the GTK C layer through do_add_or_update_menu_item
		// which always finalises with gtk_widget_show(), ensuring visibility
		// even after a preceding Hide().
		(*emptyItem).SetTitle("(empty)   ")
		(*emptyItem).Show()

		// Hide all repo slots and their children.
		for i := 0; i < m.maxNumberOfRepos; i++ {
			for j := 0; j < m.maxNumberOfPRs; j++ {
				menuItems[i].Children[j].Hide()
			}
			menuItems[i].Parent.Hide()
		}
		return
	}

	// Non-empty: hide the dedicated placeholder.
	(*emptyItem).Hide()

	prsByRepo := m.groupPRsByRepository(prs)

	// Sort repository names to ensure consistent ordering
	repoNames := make([]string, 0, len(prsByRepo))
	for repoName := range prsByRepo {
		repoNames = append(repoNames, repoName)
	}
	sort.Strings(repoNames)

	// Update repo slots that are in use
	for i, repoName := range repoNames {
		if i >= m.maxNumberOfRepos {
			break
		}
		repoPRs := prsByRepo[repoName]

		// Add asterisk to repository name if it contains unseen PRs
		repoTitle := repoName
		if m.hasUnseenPRs(repoPRs) {
			repoTitle = "* " + repoName
		}
		menuItems[i].Parent.SetTitle(repoTitle + "   ")
		menuItems[i].Parent.Enable()

		// Update PR child slots that are in use
		for j, pr := range repoPRs {
			if j >= m.maxNumberOfPRs {
				break
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
			menuItems[i].Children[j].Hide()
		}

		menuItems[i].Parent.Show()
	}

	// Hide repo slots no longer in use
	for i := len(repoNames); i < m.maxNumberOfRepos; i++ {
		menuItems[i].Parent.Hide()
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
		reviewSuffix = " " + FormatReviewSummaryForMenu(summary)
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
