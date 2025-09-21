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
	userEntry   *api.LeaderboardEntry
	loading     bool
	error       string
	language    string
	isAuthenticated bool
	user         *auth.Session
}

// Message types for async operations
type leaderboardLoadedMsg struct {
	entries   []api.LeaderboardEntry
	userEntry *api.LeaderboardEntry
}

type loadErrorMsg struct {
	error string
}

// NewLeaderboardModel creates a new leaderboard model
func NewLeaderboardModel() *LeaderboardModel {
	client := api.NewClient()
	authManager, err := auth.NewManager(client)
	if err != nil {
		// If auth manager creation fails, we'll handle it gracefully
		// by treating the user as unauthenticated
		authManager = nil
	}

	// Cache authentication status to avoid multiple HTTP requests during rendering
	var isAuthenticated bool
	var user *auth.Session
	if authManager != nil {
		isAuthenticated = authManager.IsAuthenticated()
		if isAuthenticated {
			user = authManager.GetUser()
		}
	}

	return &LeaderboardModel{
		client:          client,
		authManager:     authManager,
		loading:         true,
		language:        "english",
		isAuthenticated: isAuthenticated,
		user:            user,
	}
}

// Init initializes the leaderboard model
func (m LeaderboardModel) Init() tea.Cmd {
	return m.loadLeaderboard()
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
			return m, m.loadLeaderboard()
		}
		return m, nil

	case leaderboardLoadedMsg:
		m.entries = msg.entries
		m.userEntry = msg.userEntry
		m.loading = false
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
	header = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(header)
	sections = append(sections, header)

	// Leaderboard table
	table := m.renderLeaderboardTable()
	table = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(table)
	sections = append(sections, table)


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
	)

	// Separator
	separator := strings.Repeat("─", 48)

	var rows []string
	rows = append(rows, headerRow)
	rows = append(rows, mutedStyle.Render(separator))

	// Data rows
	for _, entry := range m.entries {
		// Highlight current user if authenticated
		style := lipgloss.NewStyle()
		if m.isAuthenticated && m.user != nil {
			if entry.GitHubID == m.user.GitHubID {
				style = style.Foreground(lipgloss.Color("11")).Bold(true)
			}
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

		row := lipgloss.JoinHorizontal(
			lipgloss.Top,
			rank, "  ", name, "  ", wpm, "  ", acc,
		)

		rows = append(rows, row)
	}

	// Add user's entry below top 10 if they're not in it and authenticated
	if m.userEntry != nil && m.isAuthenticated && m.user != nil {
		// Add separator
		separator2 := strings.Repeat("─", 48)
		rows = append(rows, mutedStyle.Render(separator2))
		
		// User's entry with highlighting
		userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
		
		rank := userStyle.Copy().Inherit(rankStyle).Render(fmt.Sprintf("#%d", m.userEntry.Rank))
		
		displayName := m.userEntry.Username
		if len(displayName) > 18 {
			displayName = displayName[:15] + "..."
		}
		name := userStyle.Copy().Inherit(nameStyle).Render(displayName)
		
		wpm := userStyle.Copy().Inherit(wpmStyle).Render(fmt.Sprintf("%.0f", m.userEntry.WPM))
		acc := userStyle.Copy().Inherit(accStyle).Render(fmt.Sprintf("%.1f%%", m.userEntry.Accuracy))
		
		userRow := lipgloss.JoinHorizontal(
			lipgloss.Top,
			rank, "  ", name, "  ", wpm, "  ", acc,
		)
		
		rows = append(rows, userRow)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}



func (m LeaderboardModel) renderInstructions() string {
	var instructions []string

	if m.isAuthenticated && m.user != nil {
		welcomeMsg := fmt.Sprintf("Logged in as %s", m.user.Username)
		instructions = append(instructions, 
			lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ " + welcomeMsg))
	} else {
		instructions = append(instructions, 
			lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("⚠ Not authenticated - scores won't be saved"))
		instructions = append(instructions, 
			mutedStyle.Render("Use 'zentype auth' to authenticate with GitHub"))
	}

	instructions = append(instructions, "")
	instructions = append(instructions, mutedStyle.Render("Press 'r' to refresh • 'q' to quit"))

    // Center the instructions across the full terminal width
    return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(
        lipgloss.JoinVertical(lipgloss.Center, instructions...),
    )
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
		mutedStyle.Copy().Align(lipgloss.Center).Render("Press 'r' to retry • 'q' to quit"),
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
		// Ensure we have a valid client
		if m.client == nil {
			return loadErrorMsg{error: "API client not initialized"}
		}
		
		response, err := m.client.GetLeaderboard(m.language)
		if err != nil {
			return loadErrorMsg{error: fmt.Sprintf("Failed to load leaderboard: %v", err)}
		}
		return leaderboardLoadedMsg{entries: response.Entries, userEntry: response.UserEntry}
	}
}


