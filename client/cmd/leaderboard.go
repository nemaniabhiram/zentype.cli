package cmd

import (
	"fmt"

	"zentype/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// leaderboardCmd represents the leaderboard command
var leaderboardCmd = &cobra.Command{
	Use:   "leaderboard",
	Short: "View the global leaderboard",
	Long: `View the global leaderboard for 60-second typing tests.
Shows the top 10 players and your current rank if you're authenticated.

To compete on the leaderboard, you need to:
- Authenticate with GitHub using 'zentype auth'
- Complete 60-second typing tests
- Achieve at least 85% accuracy`,
	Example: `  zentype leaderboard
  zentype lb`,
	Aliases: []string{"lb", "rank", "top"},
	RunE:    runLeaderboard,
}

func runLeaderboard(cmd *cobra.Command, args []string) error {
	// Create leaderboard model
	model := ui.NewLeaderboardModel()

	// Start the TUI program
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running leaderboard: %w", err)
	}

	return nil
}
