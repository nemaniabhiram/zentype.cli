package cmd

import (
	"fmt"
	"os"

	"zentype/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	version     = "v1.0.0"
	showVersion bool
	duration    int // Duration for direct typing test
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "zentype",
	Short: "A minimal typing speed test in your terminal",
	Long: `ZenType - A terminal-based typing speed test application.
	Practice your typing skills with randomized English words.`,
	Example: `  zentype --time 30
  zentype -t 60
  zentype --version`,
	Run: func(cmd *cobra.Command, args []string) {
		// If time flag is set, start the typing test directly
		if cmd.Flags().Changed("time") {
			if err := runDirectTypingTest(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		// Show help if no subcommands or flags are provided
		cmd.Help()
	},
}

// versionCmd prints the current version of zentype
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version of zentype",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("zentype version", version)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// init function initializes the root command and adds subcommands and flags
func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true // Disable default completion command

	// Add --version flag with shorthand -v
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Show the version and exit")
	
	// Add --time flag with shorthand -t for direct typing test
	rootCmd.Flags().IntVarP(&duration, "time", "t", 60, "Test duration in seconds (10-300)")

	// Add subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(leaderboardCmd)
	rootCmd.AddCommand(versionCmd)

	// Check for version flag early and exit if set
	cobra.OnInitialize(func() {
		if showVersion {
			fmt.Println("zentype version", version)
			os.Exit(0)
		}
	})
}

// runDirectTypingTest runs a typing test directly from the root command
func runDirectTypingTest() error {
	// Validate duration
	if duration < 10 || duration > 300 {
		return fmt.Errorf("duration must be between 10 and 300 seconds")
	}

	// Create a new typing test model
	model := ui.NewModel(duration, "english")

	// Start the TUI program without alternate screen for faster startup
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running typing test: %w", err)
	}

	return nil
}
