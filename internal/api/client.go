package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
	"github.com/nemaniabhiram/zentype.cli/internal/game"
)

const (
	// Default API base URL - can be overridden via environment variable
	DefaultBaseURL = "https://zentypecli-production.up.railway.app/api"
	Timeout        = 15 * time.Second
)

// LeaderboardEntry represents a leaderboard entry
type LeaderboardEntry struct {
	ID        int       `json:"id,omitempty"`
	Username  string    `json:"username"`
	GitHubID  int       `json:"github_id"`
	WPM       float64   `json:"wpm"`
	Accuracy  float64   `json:"accuracy"`
	Duration  int       `json:"duration"`
	Language  string    `json:"language"`
	CreatedAt time.Time `json:"created_at"`
	Rank      int       `json:"rank,omitempty"`
}

// UserStats represents user statistics and ranking
type UserStats struct {
	Username        string  `json:"username"`
	GitHubID        int     `json:"github_id"`
	BestWPM         float64 `json:"best_wpm"`
	BestAccuracy    float64 `json:"best_accuracy"`
	Rank            int     `json:"rank"`
	TotalScores     int     `json:"total_scores"`
	QualifiedScores int     `json:"qualified_scores"`
}

// AuthUser represents authenticated user information
type AuthUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	GitHubID int    `json:"github_id"`
	Login    string `json:"github_login"`
	Avatar   string `json:"avatar_url"`
}

// AuthData contains the URL for the user to visit to authenticate.
type AuthData struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

// Client handles API communication
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient creates a new API client
func NewClient() *Client {
	// Allow environment variable to override default URL
	baseURL := os.Getenv("ZENTYPE_API_URL")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	
	return &Client{
		httpClient: &http.Client{
			Timeout: Timeout,
		},
		baseURL: baseURL,
	}
}

// SetToken sets the authentication token
func (c *Client) SetToken(token string) {
	c.token = token
}

// GetToken returns the current authentication token
func (c *Client) GetToken() string {
	return c.token
}

// makeAuthenticatedRequest makes an HTTP request with authentication
func (c *Client) makeAuthenticatedRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequest(method, c.baseURL+endpoint, reqBody)
	} else {
		req, err = http.NewRequest(method, c.baseURL+endpoint, nil)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// CheckHealth verifies the API server is running
func (c *Client) CheckHealth() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("failed to connect to API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetAuthURL gets the GitHub OAuth authentication URL
func (c *Client) GetAuthURL() (*AuthData, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/auth/github", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get auth URL, status: %d", resp.StatusCode)
	}

	var result AuthData
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}

	return &result, nil
}

// VerifyToken verifies the authentication token and returns user info
func (c *Client) VerifyToken() (*AuthUser, error) {
	if c.token == "" {
		return nil, fmt.Errorf("no authentication token set")
	}

	resp, err := c.makeAuthenticatedRequest("GET", "/auth/verify", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid or expired token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token verification failed with status: %d", resp.StatusCode)
	}

	var user AuthUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &user, nil
}

// SubmitScore submits a typing test score to the leaderboard
func (c *Client) SubmitScore(stats game.TypingStats, duration int, language string) (*LeaderboardEntry, error) {
	if c.token == "" {
		return nil, fmt.Errorf("authentication required to submit scores")
	}

	entry := LeaderboardEntry{
		WPM:      stats.WPM,
		Accuracy: stats.Accuracy,
		Duration: duration,
		Language: language,
	}

	resp, err := c.makeAuthenticatedRequest("POST", "/scores", entry)
	if err != nil {
		return nil, fmt.Errorf("failed to submit score: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication required")
	}

	if resp.StatusCode != http.StatusCreated {
		// Try to get error message from response
		var errorResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorResp)
		if msg, ok := errorResp["error"]; ok {
			return nil, fmt.Errorf("server error: %v", msg)
		}
		return nil, fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	var result LeaderboardEntry
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// LeaderboardResponse represents the response from the leaderboard API
type LeaderboardResponse struct {
	Entries   []LeaderboardEntry `json:"entries"`
	UserEntry *LeaderboardEntry  `json:"user_entry,omitempty"`
}

// GetLeaderboard fetches the top 10 leaderboard entries and user's entry if not in top 10
func (c *Client) GetLeaderboard(language string) (*LeaderboardResponse, error) {
	if language == "" {
		language = "english"
	}

	url := fmt.Sprintf("%s/leaderboard?language=%s", c.baseURL, language)
	
	// Use authenticated request if token is available
	var resp *http.Response
	var err error
	if c.token != "" {
		resp, err = c.makeAuthenticatedRequest("GET", "/leaderboard?language="+language, nil)
	} else {
		resp, err = c.httpClient.Get(url)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to fetch leaderboard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	var response LeaderboardResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode leaderboard: %w", err)
	}

	return &response, nil
}

// GetUserRank gets the current user's ranking and statistics
func (c *Client) GetUserRank(language string) (*UserStats, error) {
	if c.token == "" {
		return nil, fmt.Errorf("authentication required to get user rank")
	}

	if language == "" {
		language = "english"
	}

	url := fmt.Sprintf("/user/rank?language=%s", language)
	resp, err := c.makeAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user rank: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication required")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	var stats UserStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode user stats: %w", err)
	}

	return &stats, nil
}

// IsAuthenticated checks if the client has a valid token
func (c *Client) IsAuthenticated() bool {
	if c.token == "" {
		return false
	}

	// Try to verify the token
	_, err := c.VerifyToken()
	return err == nil
}
