package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/nemaniabhiram/zentype.cli/internal/api"
	"github.com/nemaniabhiram/zentype.cli/internal/auth"

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
	switch runtime.GOOS {
	case "windows":
		return openBrowserWindows(url)
	case "darwin":
		return exec.Command("open", url).Start()
	default: // Linux and others
		return exec.Command("xdg-open", url).Start()
	}
}

// openBrowserWindows tries multiple methods to open a URL on Windows
func openBrowserWindows(url string) error {
	var lastErr error
	
	// Method 1: Try rundll32 first (handles long URLs better)
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err == nil {
		return nil
	} else {
		lastErr = err
	}
	
	// Method 2: Try PowerShell (handles long URLs well)
	psCmd := fmt.Sprintf("Start-Process '%s'", url)
	if err := exec.Command("powershell", "-Command", psCmd).Start(); err == nil {
		return nil
	} else {
		lastErr = err
	}
	
	// Method 3: Try cmd /c start (may truncate long URLs)
	if err := exec.Command("cmd", "/c", "start", url).Start(); err == nil {
		return nil
	} else {
		lastErr = err
	}
	
	// If all methods fail, return the last error
	return fmt.Errorf("all Windows browser opening methods failed, last error: %w", lastErr)
}
