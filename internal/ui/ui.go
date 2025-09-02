package ui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nemaniabhiram/zentype.cli/internal/game"
	"github.com/nemaniabhiram/zentype.cli/internal/api"
	"github.com/nemaniabhiram/zentype.cli/internal/auth"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const statGap = 5
const spacer = ""

// Styles for the TUI
var (
	timeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true).
			MarginLeft(8)

	textBoxStyle = lipgloss.NewStyle().
			Padding(1, 3).
			Width(60).
			Height(6).
			Align(lipgloss.Left).
			MarginLeft(5)

	boldStyle = lipgloss.NewStyle().
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true).
			Underline(true)

	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("15")).
			Foreground(lipgloss.Color("#000")).
			Bold(true)

	resultsContainerStyle = lipgloss.NewStyle().
				Padding(3, 5).
				Align(lipgloss.Left)
)

// Model represents the state of the typing test application
type Model struct {
	game        *game.TypingGame
	width       int
	height      int
	showResults bool
	finalStats  game.TypingStats
	duration    int
	language    string
	client      *api.Client
	authManager *auth.Manager
	userRank    int
	submitting  bool
	submitError string
}

// tickMsg is a message type used to handle periodic updates in the application
type tickMsg time.Time

// Message types for API operations
type scoreSubmittedMsg struct {
	entry *api.LeaderboardEntry
}

type submitErrorMsg struct {
	error string
}

type userRankMsg struct {
    rank int
}

// NewModel initializes a new Model instance with the specified duration and language
func NewModel(duration int, language string) *Model {
	client := api.NewClient()
	authManager, _ := auth.NewManager(client)
	
	return &Model{
		game:        game.NewTypingGame(duration),
		duration:    duration,
		language:    language,
		client:      client,
		authManager: authManager,
	}
}

// restartTest resets the game state for a new typing test session
func (m *Model) restartTest() {
	m.game = game.NewTypingGame(m.duration)
	m.showResults = false
	m.finalStats = game.TypingStats{}
	m.userRank = 0
	m.submitting = false
	m.submitError = ""
}

// restartCurrentTest resets the current test with the same words
func (m *Model) restartCurrentTest() {
	// Keep the same words but reset game state
	words := m.game.AllWords
	m.game = game.NewTypingGameWithWords(m.duration, words)
}

// Init initializes the model and starts the tick command for periodic updates
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

// tickCmd returns a command that sends a tick message every 1 second
func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update processes incoming messages and updates the model accordingly
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window size changes
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	// Handle keyboard input and game logic
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			if m.showResults {
				m.restartTest()
				return m, tickCmd()
			}
			// If game has started, restart current test
			if m.game.IsStarted {
				m.restartCurrentTest()
				return m, tickCmd()
			}
			// Handle Enter for line progression if no input yet
			if m.game.HandleEnterKey() {
				return m, nil
			}
			return m, nil

		case " ":
			if !m.showResults && !m.game.IsFinished && !m.game.IsTimeUp() {
				m.game.AddCharacter(' ')
			}
			return m, nil

		case "backspace":
			if !m.showResults && !m.game.IsFinished {
				m.game.RemoveCharacter()
			}
			return m, nil

		default:
			// Handle regular character input
			if !m.showResults && !m.game.IsFinished && !m.game.IsTimeUp() {
				runes := []rune(msg.String())
				if len(runes) == 1 && runes[0] >= 32 && runes[0] <= 126 {
					m.game.AddCharacter(runes[0])
				}
			}
			return m, nil
		}

	// Handle tick messages for periodic updates
	case tickMsg:
		if !m.showResults {
			if m.game.IsTimeUp() && m.game.IsStarted {
				m.finalStats = m.game.GetStats()
				m.showResults = true
				
				// Submit score if authenticated and 60-second test
				if m.authManager.IsAuthenticated() && m.duration == 60 && !m.submitting {
					log.Printf("DEBUG: User authenticated, submitting score for 60s test")
					m.submitting = true
					return m, m.submitScore()
				} else {
					log.Printf("DEBUG: Not submitting score - authenticated: %v, duration: %d, submitting: %v", 
						m.authManager.IsAuthenticated(), m.duration, m.submitting)
				}
				
				return m, nil
			}
			return m, tickCmd()
		}
		return m, nil

	// Handle score submission results
	case scoreSubmittedMsg:
        m.submitting = false
        log.Printf("DEBUG: Score submitted, entry: %+v", msg.entry)
        if msg.entry != nil {
            m.userRank = msg.entry.Rank
            log.Printf("DEBUG: Set userRank from entry: %d", m.userRank)
        }
        if m.userRank == 0 {
            log.Printf("DEBUG: userRank is 0, fetching rank...")
            return m, m.getRankCmd()
        }
        return m, nil

	case userRankMsg:
        log.Printf("DEBUG: Received userRankMsg with rank: %d", msg.rank)
        if msg.rank > 0 {
            m.userRank = msg.rank
            log.Printf("DEBUG: Updated userRank to: %d", m.userRank)
        }
        return m, nil

    case submitErrorMsg:
		m.submitting = false
		m.submitError = msg.error
		return m, nil
	}

	return m, nil
}

// View renders the current state of the Model as a string for display
func (m Model) View() string {
	if m.showResults {
		return m.renderResults()
	}

	var sections []string

	timer := m.renderTimer()
	sections = append(sections, timer)

	textDisplay := m.renderText()
	sections = append(sections, textDisplay)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// renderTimer formats the remaining time for display
func (m Model) renderTimer() string {
	remaining := m.game.GetRemainingTime()
	return timeStyle.Render(fmt.Sprintf("%d", remaining))
}

// renderText formats the text display with appropriate styles for typed, current, untyped characters
func (m Model) renderText() string {
	displayText := m.game.GetDisplayText()

	var rendered strings.Builder

	for i, char := range displayText {
		// Use helper to style character
		styledChar := m.styleChar(char, i)
		rendered.WriteString(styledChar)
	}

	// Format into lines
	content := rendered.String()
	lines := m.formatIntoLines(content)

	return textBoxStyle.Render(strings.Join(lines, "\n"))
}

// formatIntoLines formats the content into lines based on the game's display settings
func (m Model) formatIntoLines(plainContent string) []string {
	lines := m.game.DisplayLines

	maxLines := m.game.LinesPerView
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	var styledLines []string
	charIndex := 0

	for i, line := range lines {
		if i >= maxLines {
			break
		}

		var styledLine strings.Builder

		lineRunes := []rune(line)

		for col := 0; col < len(lineRunes); col++ {
			if charIndex < len(plainContent) {
				styledChar := m.styleChar(lineRunes[col], charIndex)
				styledLine.WriteString(styledChar)
				charIndex++
			} else {
				styledLine.WriteString(mutedStyle.Render(string(lineRunes[col])))
			}
		}

		// Check if caret is on this line and positioned just beyond last char
		caretPos := m.game.CurrentPos
		if i == 0 && caretPos == len(lineRunes) {
			// Append caret style with a space or block to show cursor
			styledLine.WriteString(cursorStyle.Render(" "))
		}

		styledLines = append(styledLines, styledLine.String())

		// Advance charIndex as before for spacing between lines
		if charIndex < len(plainContent) && i < len(lines)-1 {
			charIndex++
		}
	}

	return styledLines
}

// styleChar determines the style of a character based on its position and error status
func (m Model) styleChar(char rune, index int) string {
	userPos := m.game.CurrentPos
	errorIndex := m.game.GlobalPos - (userPos - index)

	switch {
	case index < userPos:
		// Already typed
		if m.game.Errors != nil {
			if _, hasErr := m.game.Errors[errorIndex]; hasErr {
				return errorStyle.Render(string(char))
			}
		}
		return boldStyle.Render(string(char))
	case index == userPos:
		// Current character
		return cursorStyle.Render(string(char))
	default:
		// Not yet typed
		return mutedStyle.Render(string(char))
	}
}

// renderResults formats the final results of the typing test for display
func (m Model) renderResults() string {
	stats := m.finalStats

	accSection := lipgloss.JoinVertical(
		lipgloss.Right,
		mutedStyle.Render("acc"),
		boldStyle.Render(fmt.Sprintf("%.0f%%", stats.Accuracy)),
	)

	wpmSection := lipgloss.JoinVertical(
		lipgloss.Right,
		mutedStyle.Render("wpm"),
		boldStyle.Render(fmt.Sprintf("%.0f", stats.WPM)),
	)

	timeSection := lipgloss.JoinVertical(
		lipgloss.Right,
		mutedStyle.Render("time"),
		boldStyle.Render(fmt.Sprintf("%.0fs", stats.TimeElapsed.Seconds())),
	)

	languageSection := lipgloss.JoinVertical(
		lipgloss.Right,
		mutedStyle.Render("lang"),
		boldStyle.Render(m.language),
	)

	// Add rank section for 60-second tests
	var rankSection string
	if m.duration == 60 {
		if m.submitting {
			rankSection = lipgloss.JoinVertical(
				lipgloss.Right,
				mutedStyle.Render("rank"),
				boldStyle.Render("..."),
			)
		} else if m.userRank > 0 {
			rankText := fmt.Sprintf("#%d", m.userRank)
			if m.userRank <= 10 {
				rankText = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(rankText)
			} else {
				rankText = boldStyle.Render(rankText)
			}
			rankSection = lipgloss.JoinVertical(
				lipgloss.Right,
				mutedStyle.Render("rank"),
				rankText,
			)
		} else if m.submitError != "" {
			rankSection = lipgloss.JoinVertical(
				lipgloss.Right,
				mutedStyle.Render("rank"),
				lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("error"),
			)
		} else if !m.authManager.IsAuthenticated() {
			log.Printf("DEBUG: User not authenticated, showing n/a")
			rankSection = lipgloss.JoinVertical(
				lipgloss.Right,
				mutedStyle.Render("rank"),
				mutedStyle.Render("n/a"),
			)
		} else if m.userRank == 0 {
            log.Printf("DEBUG: userRank is 0, showing n/a")
            rankSection = lipgloss.JoinVertical(
                lipgloss.Right,
                mutedStyle.Render("rank"),
                mutedStyle.Render("n/a"),
            )
        } else if stats.Accuracy < 85.0 {
			rankSection = lipgloss.JoinVertical(
				lipgloss.Right,
				mutedStyle.Render("rank"),
				mutedStyle.Render("85%+"),
			)
		}
	}

	// Arrange stats horizontally
	var statsRow string
	if rankSection != "" {
		statsRow = lipgloss.JoinHorizontal(
			lipgloss.Top,
			accSection,
			strings.Repeat(" ", statGap),
			wpmSection,
			strings.Repeat(" ", statGap),
			timeSection,
			strings.Repeat(" ", statGap),
			languageSection,
			strings.Repeat(" ", statGap),
			rankSection,
		)
	} else {
		statsRow = lipgloss.JoinHorizontal(
			lipgloss.Top,
			accSection,
			strings.Repeat(" ", statGap),
			wpmSection,
			strings.Repeat(" ", statGap),
			timeSection,
			strings.Repeat(" ", statGap),
			languageSection,
		)
	}

	instructions := mutedStyle.Align(lipgloss.Center).Render("Press Enter to restart • Esc to quit")

	// Results layout
	resultsContent := lipgloss.JoinVertical(
		lipgloss.Center,
		spacer,
		statsRow,
		spacer,
		instructions,
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		resultsContainerStyle.Render(resultsContent),
	)
}

// getRankCmd fetches the user's rank from the server
func (m Model) getRankCmd() tea.Cmd {
    return func() tea.Msg {
        log.Printf("DEBUG: Fetching user rank for language: %s", m.language)
        if stats, err := m.client.GetUserRank(m.language); err == nil {
            log.Printf("DEBUG: GetUserRank success, rank: %d", stats.Rank)
            return userRankMsg{rank: stats.Rank}
        } else {
            log.Printf("DEBUG: GetUserRank error: %v", err)
        }
        return userRankMsg{rank: 0}
    }
}

// submitScore submits the user's score to the leaderboard
func (m Model) submitScore() tea.Cmd {
    return func() tea.Msg {
        log.Printf("DEBUG: Submitting score - WPM: %.1f, Accuracy: %.1f, Duration: %d", m.finalStats.WPM, m.finalStats.Accuracy, m.duration)
        entry, err := m.client.SubmitScore(m.finalStats, m.duration, m.language)
        if err != nil {
            log.Printf("DEBUG: SubmitScore error: %v", err)
            return submitErrorMsg{error: err.Error()}
        }
        log.Printf("DEBUG: SubmitScore success, entry: %+v", entry)
        // Always refresh rank after submission (server may calculate asynchronously)
        if stats, err := m.client.GetUserRank(m.language); err == nil {
            log.Printf("DEBUG: GetUserRank in submitScore success, rank: %d", stats.Rank)
            if entry == nil {
                entry = &api.LeaderboardEntry{}
            }
            entry.Rank = stats.Rank
        } else {
            log.Printf("DEBUG: GetUserRank in submitScore error: %v", err)
        }
        return scoreSubmittedMsg{entry: entry}
    }
}