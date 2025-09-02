package ui

import (
	"fmt"
	"strings"
	"time"
	"github.com/nemaniabhiram/zentype.cli/internal/api"
	"github.com/nemaniabhiram/zentype.cli/internal/auth"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LeaderboardModel represents the leaderboard screen
type LeaderboardModel struct {
	width       int
	height      int
	client      *api.Client
	authManager *auth.Manager
	entries     []api.LeaderboardEntry
	userStats   *api.UserStats
	loading     bool
	error       string
	language    string
}

// Message types for async operations
type leaderboardLoadedMsg struct {
	entries []api.LeaderboardEntry
}

type userStatsLoadedMsg struct {
	stats *api.UserStats
}

type loadErrorMsg struct {
	error string
}

// NewLeaderboardModel creates a new leaderboard model
func NewLeaderboardModel() *LeaderboardModel {
	client := api.NewClient()
	authManager, _ := auth.NewManager(client)

	return &LeaderboardModel{
		client:      client,
		authManager: authManager,
		loading:     true,
		language:    "english",
	}
}

// Init initializes the leaderboard model
func (m LeaderboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadLeaderboard(),
		m.loadUserStats(),
	)
}

// Update handles messages for the leaderboard
func (m LeaderboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "r", "f5":
			// Refresh leaderboard
			m.loading = true
			m.error = ""
			return m, tea.Batch(
				m.loadLeaderboard(),
				m.loadUserStats(),
			)
		case "a":
			// Show authentication info/help
			return m, m.showAuthHelp()
		}
		return m, nil

	case leaderboardLoadedMsg:
		m.entries = msg.entries
		m.loading = false
		return m, nil

	case userStatsLoadedMsg:
		m.userStats = msg.stats
		return m, nil

	case loadErrorMsg:
		m.error = msg.error
		m.loading = false
		return m, nil
	}

	return m, nil
}

// View renders the leaderboard screen
func (m LeaderboardModel) View() string {
	if m.loading {
		return m.renderLoading()
	}

	if m.error != "" {
		return m.renderError()
	}

	var sections []string

	// Header
	header := m.renderHeader()
	sections = append(sections, header)

	// Leaderboard table
	table := m.renderLeaderboardTable()
	sections = append(sections, table)

	// User stats (if authenticated)
	if m.userStats != nil {
		userSection := m.renderUserStats()
		sections = append(sections, userSection)
	}

	// Instructions
	instructions := m.renderInstructions()
	sections = append(sections, instructions)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m LeaderboardModel) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Align(lipgloss.Center).
		Render("🏆 ZenType Global Leaderboard")

	subtitle := mutedStyle.Align(lipgloss.Center).
		Render("60-second tests • Minimum 85% accuracy • English words")

	return lipgloss.JoinVertical(lipgloss.Center, title, "", subtitle)
}

func (m LeaderboardModel) renderLeaderboardTable() string {
	if len(m.entries) == 0 {
		return mutedStyle.Align(lipgloss.Center).Render("No leaderboard entries found")
	}

	// Table styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")).
		Align(lipgloss.Center)

	rankStyle := lipgloss.NewStyle().
		Width(4).
		Align(lipgloss.Right)

	nameStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Left)

	wpmStyle := lipgloss.NewStyle().
		Width(8).
		Align(lipgloss.Right)

	accStyle := lipgloss.NewStyle().
		Width(8).
		Align(lipgloss.Right)

	dateStyle := lipgloss.NewStyle().
		Width(12).
		Align(lipgloss.Center)

	// Header row
	headerRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		headerStyle.Copy().Inherit(rankStyle).Render("Rank"),
		"  ",
		headerStyle.Copy().Inherit(nameStyle).Render("Player"),
		"  ",
		headerStyle.Copy().Inherit(wpmStyle).Render("WPM"),
		"  ",
		headerStyle.Copy().Inherit(accStyle).Render("Accuracy"),
		"  ",
		headerStyle.Copy().Inherit(dateStyle).Render("Date"),
	)

	// Separator
	separator := strings.Repeat("─", 60)

	var rows []string
	rows = append(rows, headerRow)
	rows = append(rows, mutedStyle.Render(separator))

	// Data rows
	for _, entry := range m.entries {
		// Highlight current user if authenticated
		style := lipgloss.NewStyle()
		if m.userStats != nil && entry.GitHubID == m.userStats.GitHubID {
			style = style.Foreground(lipgloss.Color("11")).Bold(true)
		}

		rank := style.Copy().Inherit(rankStyle).Render(fmt.Sprintf("#%d", entry.Rank))
		
		// Truncate long usernames
		displayName := entry.Username
		if len(displayName) > 18 {
			displayName = displayName[:15] + "..."
		}
		name := style.Copy().Inherit(nameStyle).Render(displayName)
		
		wpm := style.Copy().Inherit(wpmStyle).Render(fmt.Sprintf("%.0f", entry.WPM))
		acc := style.Copy().Inherit(accStyle).Render(fmt.Sprintf("%.1f%%", entry.Accuracy))
		
		// Format date
		dateStr := entry.CreatedAt.Format("Jan 02")
		if entry.CreatedAt.Year() != time.Now().Year() {
			dateStr = entry.CreatedAt.Format("Jan 2006")
		}
		date := style.Copy().Inherit(dateStyle).Render(dateStr)

		row := lipgloss.JoinHorizontal(
			lipgloss.Top,
			rank, "  ", name, "  ", wpm, "  ", acc, "  ", date,
		)

		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m LeaderboardModel) renderUserStats() string {
	if m.userStats == nil {
		return ""
	}

	title := boldStyle.Render("Your Statistics")

	var stats []string

	// Best score
	bestScore := fmt.Sprintf("Best: %.0f WPM • %.1f%% accuracy", 
		m.userStats.BestWPM, m.userStats.BestAccuracy)
	stats = append(stats, bestScore)

	// Rank
	var rankStr string
	if m.userStats.Rank > 0 {
		rankStr = fmt.Sprintf("Global Rank: #%d", m.userStats.Rank)
		if m.userStats.Rank <= 10 {
			rankStr = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(rankStr)
		}
	} else {
		if m.userStats.QualifiedScores == 0 {
			rankStr = mutedStyle.Render("Not ranked (need 85%+ accuracy)")
		} else {
			rankStr = mutedStyle.Render("Not ranked")
		}
	}
	stats = append(stats, rankStr)

	// Test count
	testInfo := fmt.Sprintf("Tests: %d total • %d qualified", 
		m.userStats.TotalScores, m.userStats.QualifiedScores)
	stats = append(stats, mutedStyle.Render(testInfo))

	content := lipgloss.JoinVertical(lipgloss.Left, 
		title,
		"",
		strings.Join(stats, "\n"),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(1, 2).
		MarginTop(2).
		Render(content)
}

func (m LeaderboardModel) renderInstructions() string {
	var instructions []string

	if m.authManager.IsAuthenticated() {
		user := m.authManager.GetUser()
		welcomeMsg := fmt.Sprintf("Logged in as %s", user.Username)
		instructions = append(instructions, 
			lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ " + welcomeMsg))
	} else {
		instructions = append(instructions, 
			lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("⚠ Not authenticated - scores won't be saved"))
		instructions = append(instructions, 
			mutedStyle.Render("Use 'zentype auth' to authenticate with GitHub"))
	}

	instructions = append(instructions, "")
	instructions = append(instructions, mutedStyle.Render("Press 'r' to refresh • 'a' for auth help • 'q' to quit"))

	return lipgloss.JoinVertical(lipgloss.Center, instructions...)
}

func (m LeaderboardModel) renderLoading() string {
	spinner := "⣾⣽⣻⢿⡿⣟⣯⣷"
	frame := int(time.Now().UnixMilli()/100) % len(spinner)
	
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(string(spinner[frame])+" Loading leaderboard..."),
		"",
		mutedStyle.Render("Fetching the latest rankings..."),
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m LeaderboardModel) renderError() string {
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).Render("❌ Error Loading Leaderboard"),
		"",
		mutedStyle.Render(m.error),
		"",
		mutedStyle.Render("Press 'r' to retry • 'q' to quit"),
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// loadLeaderboard loads the leaderboard data
func (m LeaderboardModel) loadLeaderboard() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.client.GetLeaderboard(m.language)
		if err != nil {
			return loadErrorMsg{error: fmt.Sprintf("Failed to load leaderboard: %v", err)}
		}
		return leaderboardLoadedMsg{entries: entries}
	}
}

// loadUserStats loads the user's statistics if authenticated
func (m LeaderboardModel) loadUserStats() tea.Cmd {
	return func() tea.Msg {
		if !m.authManager.IsAuthenticated() {
			return userStatsLoadedMsg{stats: nil}
		}

		stats, err := m.client.GetUserRank(m.language)
		if err != nil {
			// Don't treat user stats error as fatal
			return userStatsLoadedMsg{stats: nil}
		}
		return userStatsLoadedMsg{stats: stats}
	}
}

// showAuthHelp shows authentication help
func (m LeaderboardModel) showAuthHelp() tea.Cmd {
	return func() tea.Msg {
		// This could open a help screen or show auth instructions
		// For now, just return nil to keep it simple
		return nil
	}
}
