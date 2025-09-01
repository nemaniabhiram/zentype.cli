package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"zentype/internal/api"
	"zentype/internal/auth"

	"github.com/spf13/cobra"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with GitHub for leaderboard access",
	Long: `Authenticate with GitHub to submit scores to the global leaderboard.

This command will:
1. Open GitHub OAuth in your browser
2. Guide you through the authentication process
3. Save your authentication token locally

Your GitHub account will be used as your leaderboard identity.
Only 60-second tests with 85%+ accuracy will be submitted to the leaderboard.`,
	Example: `  zentype auth
  zentype auth --logout
  zentype auth --status`,
	RunE: runAuth,
}

var (
	authLogout bool
	authStatus bool
)

func init() {
	authCmd.Flags().BoolVar(&authLogout, "logout", false, "Logout and clear saved authentication")
	authCmd.Flags().BoolVar(&authStatus, "status", false, "Show current authentication status")
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) error {
	client := api.NewClient()
	authManager, err := auth.NewManager(client)
	if err != nil {
		return fmt.Errorf("failed to initialize auth manager: %w", err)
	}

	// Handle logout
	if authLogout {
		if !authManager.IsAuthenticated() {
			fmt.Println("✓ You are not currently authenticated")
			return nil
		}

		if err := authManager.Logout(); err != nil {
			return fmt.Errorf("failed to logout: %w", err)
		}

		fmt.Println("✓ Successfully logged out")
		return nil
	}

	// Handle status check
	if authStatus {
		if authManager.IsAuthenticated() {
			user := authManager.GetUser()
			fmt.Printf("✓ Authenticated as: %s (@%s)\n", user.Username, user.GitHubLogin)
			fmt.Printf("  GitHub ID: %d\n", user.GitHubID)
			fmt.Printf("  Authenticated: %s\n", user.CreatedAt.Format("Jan 2, 2006 15:04"))
			
			// Test API connection
			if err := client.CheckHealth(); err != nil {
				fmt.Printf("  ⚠ API Status: Offline (%v)\n", err)
			} else {
				fmt.Printf("  ✓ API Status: Online\n")
			}
		} else {
			fmt.Println("✗ Not authenticated")
			fmt.Println("  Run 'zentype auth' to authenticate with GitHub")
		}
		return nil
	}

	// Check if already authenticated
	if authManager.IsAuthenticated() {
		user := authManager.GetUser()
		fmt.Printf("✓ Already authenticated as %s (@%s)\n", user.Username, user.GitHubLogin)
		fmt.Println("  Use 'zentype auth --logout' to logout")
		fmt.Println("  Use 'zentype auth --status' for more details")
		return nil
	}

	// Start authentication flow
	fmt.Println("🔐 Starting GitHub authentication...")
	fmt.Println()

	// Check API health first
	if err := client.CheckHealth(); err != nil {
		fmt.Printf("❌ Cannot connect to ZenType API server\n")
		fmt.Printf("Error: %v\n", err)
		fmt.Println()
		fmt.Println("Make sure the API server is running:")
		fmt.Println("  zentype server")
		return fmt.Errorf("API server unavailable")
	}

	// Get auth URL
	authData, err := client.GetAuthURL()
	if err != nil {
		return fmt.Errorf("failed to get authentication URL: %w", err)
	}

	fmt.Println("📱 Opening GitHub OAuth in your browser...")
	fmt.Println("If the browser doesn't open automatically, copy this URL:")
	fmt.Printf("\n%s\n\n", authData.AuthURL)

	// Try to open browser
	if err := openBrowser(authData.AuthURL); err != nil {
		fmt.Printf("⚠ Could not open browser automatically: %v\n", err)
		fmt.Println("Please copy and paste the full URL above into your browser")
	}

	fmt.Println("👀 Complete the authentication in your browser")
	fmt.Println("📋 Copy the token from the success page and paste it below")
	fmt.Println()

	// Prompt for token input
	fmt.Print("🔑 Enter your authentication token: ")
	var token string
	if _, err := fmt.Scanln(&token); err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}

	// Set the token
	fmt.Println("🔄 Verifying token with server...")
	if err := authManager.SetToken(token); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	fmt.Println("💾 Token saved successfully")

	// Get user info to confirm
	user := authManager.GetUser()
	fmt.Println()
	fmt.Printf("✅ Successfully authenticated!\n")
	fmt.Printf("   Welcome, %s (@%s)\n", user.Username, user.GitHubLogin)
	fmt.Println()
	fmt.Println("🎯 You can now compete on the global leaderboard!")
	fmt.Println("   Run 'zentype start -t 60' to play a ranked game")
	fmt.Println("   Run 'zentype leaderboard' to view the rankings")

	return nil
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", "", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // Linux and others
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
