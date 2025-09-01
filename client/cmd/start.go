package cmd

import (
	"fmt"

	"zentype/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	startDuration int // Duration of the typing test in seconds
)

// startCmd represents the start command for the typing test
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a typing test",
	Long:  "Start a new typing test session with customizable duration",
	Example: `  zentype start --time 60
  zentype start -t 30
  zentype start`,
	RunE: runTypingTest,
}

func init() {
	startCmd.Flags().IntVarP(&startDuration, "time", "t", 60, "Test duration in seconds (10-300)")
}

// runTypingTest runs the typing test
func runTypingTest(cmd *cobra.Command, args []string) error {
	// Validate duration
	if startDuration < 10 || startDuration > 300 {
		return fmt.Errorf("duration must be between 10 and 300 seconds (e.g., --time 60)")
	}

	// Create a new typing test model
	model := ui.NewModel(startDuration, "english")

	// Start the TUI program without alternate screen for faster startup
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running typing test: %w", err)
	}

	return nil
}
