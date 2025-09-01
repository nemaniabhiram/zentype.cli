package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"zentype/internal/api"
)

// Session represents a user authentication session
type Session struct {
	Token       string    `json:"token"`
	Username    string    `json:"username"`
	GitHubID    int       `json:"github_id"`
	GitHubLogin string    `json:"github_login"`
	Avatar      string    `json:"avatar_url"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// Manager handles user authentication and session management
type Manager struct {
	client      *api.Client
	session     *Session
	configPath  string
}

// NewManager creates a new authentication manager
func NewManager(client *api.Client) (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".zentype")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	manager := &Manager{
		client:     client,
		configPath: filepath.Join(configDir, "auth.json"),
	}

	// Try to load existing session
	if err := manager.loadSession(); err == nil {
		// Verify the session is still valid
		if manager.isSessionValid() {
			client.SetToken(manager.session.Token)
		} else {
			// Session expired, remove it
			manager.clearSession()
		}
	}

	return manager, nil
}

// IsAuthenticated checks if the user is authenticated
func (m *Manager) IsAuthenticated() bool {
	return m.session != nil && m.isSessionValid()
}

// GetUser returns the current authenticated user info
func (m *Manager) GetUser() *Session {
	if !m.IsAuthenticated() {
		return nil
	}
	return m.session
}

// SetToken manually sets an authentication token (from OAuth flow)
func (m *Manager) SetToken(token string) error {
	m.client.SetToken(token)

	// Verify the token and get user info
	user, err := m.client.VerifyToken()
	if err != nil {
		m.client.SetToken("") // Clear invalid token
		return fmt.Errorf("invalid token: %w", err)
	}

	// Create new session
	session := &Session{
		Token:       token,
		Username:    user.Username,
		GitHubID:    user.GitHubID,
		GitHubLogin: user.Login,
		Avatar:      user.Avatar,
		ExpiresAt:   time.Now().AddDate(0, 1, 0), // Expire in 1 month
		CreatedAt:   time.Now(),
	}

	m.session = session
	return m.saveSession()
}

// GetAuthURL returns the GitHub OAuth URL for authentication
func (m *Manager) GetAuthURL() (string, error) {
	return m.client.GetAuthURL()
}

// Logout clears the current session
func (m *Manager) Logout() error {
	m.session = nil
	m.client.SetToken("")
	return m.clearSession()
}

// loadSession loads the session from disk
func (m *Manager) loadSession() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return err
	}

	m.session = &session
	return nil
}

// saveSession saves the current session to disk
func (m *Manager) saveSession() error {
	if m.session == nil {
		return fmt.Errorf("no session to save")
	}

	data, err := json.MarshalIndent(m.session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// clearSession removes the session file
func (m *Manager) clearSession() error {
	m.session = nil
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to clear
	}
	return os.Remove(m.configPath)
}

// isSessionValid checks if the current session is valid and not expired
func (m *Manager) isSessionValid() bool {
	if m.session == nil {
		return false
	}

	// Check if token is expired
	if time.Now().After(m.session.ExpiresAt) {
		return false
	}

	// Verify with the server (this could be cached for performance)
	_, err := m.client.VerifyToken()
	return err == nil
}

// RefreshUserInfo updates the user information from the server
func (m *Manager) RefreshUserInfo() error {
	if !m.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}

	user, err := m.client.VerifyToken()
	if err != nil {
		return fmt.Errorf("failed to refresh user info: %w", err)
	}

	// Update session with fresh data
	m.session.Username = user.Username
	m.session.GitHubLogin = user.Login
	m.session.Avatar = user.Avatar

	return m.saveSession()
}
