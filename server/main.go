package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
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

// APIServer handles all HTTP requests
type APIServer struct {
	db          *sql.DB
	oauthConfig *oauth2.Config
}

const (
	MinAccuracy    = 85.0 // Minimum accuracy to get on leaderboard
	TargetDuration = 60   // Only 60-second tests count
)

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	log.Println("🚀 Starting ZenType API Server...")

	// Database connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL environment variable is required")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("❌ Failed to connect to database:", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatal("❌ Failed to ping database:", err)
	}
	log.Println("✅ Connected to PostgreSQL database")

	// Initialize database
	if err := initDB(db); err != nil {
		log.Fatal("❌ Failed to initialize database:", err)
	}
	log.Println("✅ Database schema initialized")

	// OAuth configuration
	oauthConfig := &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint,
		RedirectURL:  getRedirectURL(),
	}

	if oauthConfig.ClientID == "" || oauthConfig.ClientSecret == "" {
		log.Fatal("❌ GitHub OAuth credentials not configured. Set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET")
	}
	log.Printf("✅ GitHub OAuth configured (Client ID: %s...)", oauthConfig.ClientID[:8])

	server := &APIServer{
		db:          db,
		oauthConfig: oauthConfig,
	}

	// Setup routes
	r := mux.NewRouter()
	api := r.PathPrefix("/api").Subrouter()

	// CORS middleware - allow all origins for global client access
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		handlers.AllowCredentials(),
	)

	// Health and info endpoints
	api.HandleFunc("/health", server.healthCheck).Methods("GET")
	api.HandleFunc("/info", server.serverInfo).Methods("GET")

	// Authentication endpoints
	api.HandleFunc("/auth/github", server.githubAuth).Methods("GET")
	api.HandleFunc("/auth/github/callback", server.githubCallback).Methods("GET")
	api.HandleFunc("/auth/verify", server.verifyToken).Methods("GET")

	// Leaderboard endpoints
	api.HandleFunc("/scores", server.submitScore).Methods("POST")
	api.HandleFunc("/leaderboard", server.getLeaderboard).Methods("GET")
	api.HandleFunc("/user/rank", server.getUserRank).Methods("GET")

	// Statistics endpoints
	api.HandleFunc("/stats", server.getGlobalStats).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🌐 Server starting on port %s", port)
	apiBaseURL := fmt.Sprintf("http://localhost:%s", port)
	if railwayDomain := os.Getenv("RAILWAY_PUBLIC_DOMAIN"); railwayDomain != "" {
		apiBaseURL = "https" + "://" + railwayDomain
	}
	log.Printf("📝 API Base URL: %s/api", apiBaseURL)
	log.Printf("🔐 OAuth Redirect: %s", oauthConfig.RedirectURL)
	log.Printf("🎯 Leaderboard Rules: %ds tests, %.0f%% min accuracy", TargetDuration, MinAccuracy)
	log.Println("✨ Ready to serve ZenType clients!")

	if err := http.ListenAndServe(":"+port, corsHandler(r)); err != nil {
		log.Fatal("❌ Server failed to start:", err)
	}
}

func getRedirectURL() string {
	// Use GITHUB_REDIRECT_URL if explicitly set
	if url := os.Getenv("GITHUB_REDIRECT_URL"); url != "" {
		return url
	}

	// Use Railway's public domain if available
	if railwayDomain := os.Getenv("RAILWAY_PUBLIC_DOMAIN"); railwayDomain != "" {
		return fmt.Sprintf("https://%s/api/auth/github/callback", railwayDomain)
	}

	// Default for local development
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return fmt.Sprintf("http://localhost:%s/api/auth/github/callback", port)
}

func initDB(db *sql.DB) error {
	schema := `
	-- Users table with GitHub integration
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL,
		github_id INTEGER UNIQUE NOT NULL,
		github_login VARCHAR(50) NOT NULL,
		avatar_url TEXT,
		access_token TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Scores table for 60-second tests only
	CREATE TABLE IF NOT EXISTS scores (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id),
		username VARCHAR(50) NOT NULL,
		github_id INTEGER NOT NULL,
		wpm DECIMAL(6,2) NOT NULL CHECK (wpm >= 0 AND wpm <= 300),
		accuracy DECIMAL(5,2) NOT NULL CHECK (accuracy >= 0 AND accuracy <= 100),
		duration INTEGER NOT NULL DEFAULT 60 CHECK (duration = 60),
		language VARCHAR(20) NOT NULL DEFAULT 'english',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Indexes for fast leaderboard queries
	CREATE INDEX IF NOT EXISTS idx_scores_leaderboard 
	ON scores(wpm DESC, accuracy DESC, created_at DESC) 
	WHERE accuracy >= 85.0 AND duration = 60;
	
	CREATE INDEX IF NOT EXISTS idx_scores_user_rank 
	ON scores(github_id, created_at DESC);
	
	CREATE INDEX IF NOT EXISTS idx_users_github_id 
	ON users(github_id);

	-- Function to update user updated_at timestamp
	CREATE OR REPLACE FUNCTION update_user_updated_at()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = CURRENT_TIMESTAMP;
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;

	-- Trigger for updated_at
	DROP TRIGGER IF EXISTS update_user_updated_at_trigger ON users;
	CREATE TRIGGER update_user_updated_at_trigger
		BEFORE UPDATE ON users
		FOR EACH ROW
		EXECUTE FUNCTION update_user_updated_at();
	`

	_, err := db.Exec(schema)
	return err
}

func (s *APIServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "OK",
		"timestamp":  time.Now(),
		"version":    "1.0.0",
		"service":    "zentype-server",
	})
}

func (s *APIServer) serverInfo(w http.ResponseWriter, r *http.Request) {
	// Get some basic stats
	var totalUsers, totalScores int
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	s.db.QueryRow("SELECT COUNT(*) FROM scores WHERE accuracy >= $1 AND duration = $2", MinAccuracy, TargetDuration).Scan(&totalScores)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":         "ZenType Leaderboard API",
		"version":         "1.0.0",
		"min_accuracy":    MinAccuracy,
		"target_duration": TargetDuration,
		"total_users":     totalUsers,
		"total_scores":    totalScores,
		"features": []string{
			"github_oauth",
			"global_leaderboard", 
			"user_rankings",
			"60s_typing_tests",
		},
	})
}

func (s *APIServer) githubAuth(w http.ResponseWriter, r *http.Request) {
	state := fmt.Sprintf("zentype_%d", time.Now().Unix())
	url := s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"auth_url": url,
		"state":    state,
	})
}

func (s *APIServer) githubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No code provided", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := s.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	// Get user info from GitHub
	client := s.oauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var githubUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		http.Error(w, "Failed to decode user info", http.StatusInternalServerError)
		return
	}

	// Use GitHub login as username, fallback to name
	username := githubUser.Login
	if githubUser.Name != "" {
		username = githubUser.Name
	}

	// Store/update user in database
	var userID int
	err = s.db.QueryRow(`
		INSERT INTO users (username, github_id, github_login, avatar_url, access_token) 
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (github_id) 
		DO UPDATE SET 
			username = EXCLUDED.username,
			github_login = EXCLUDED.github_login,
			avatar_url = EXCLUDED.avatar_url,
			access_token = EXCLUDED.access_token,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id`,
		username, githubUser.ID, githubUser.Login, githubUser.AvatarURL, token.AccessToken,
	).Scan(&userID)

	if err != nil {
		http.Error(w, "Failed to store user", http.StatusInternalServerError)
		return
	}

	// Return success page with token
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>ZenType - Authentication Success</title>
			<style>
				body { 
					font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; 
					text-align: center; 
					padding: 50px; 
					background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
					color: white;
					margin: 0;
				}
				.container { 
					max-width: 500px; 
					margin: 0 auto; 
					background: rgba(255,255,255,0.95); 
					padding: 40px; 
					border-radius: 15px; 
					box-shadow: 0 10px 30px rgba(0,0,0,0.3);
					color: #333;
				}
				.success { color: #22c55e; font-size: 48px; margin-bottom: 20px; }
				h1 { color: #333; margin-bottom: 10px; font-size: 28px; }
				.user-info { 
					background: #f8f9fa; 
					padding: 20px; 
					border-radius: 10px; 
					margin: 20px 0; 
					border-left: 4px solid #22c55e;
				}
				.avatar { 
					width: 80px; 
					height: 80px; 
					border-radius: 50%%; 
					margin: 0 auto 15px; 
					display: block; 
					border: 3px solid #22c55e;
				}
				.token { 
					font-family: 'Monaco', 'Consolas', monospace; 
					background: #2d3748; 
					color: #e2e8f0;
					padding: 15px; 
					border-radius: 8px; 
					word-break: break-all; 
					font-size: 14px;
					margin: 15px 0;
				}
				.copy-btn {
					background: #4f46e5;
					color: white;
					border: none;
					padding: 10px 20px;
					border-radius: 6px;
					cursor: pointer;
					font-size: 14px;
					margin-top: 10px;
				}
				.copy-btn:hover { background: #4338ca; }
				.instructions { 
					color: #6b7280; 
					font-size: 14px; 
					margin-top: 20px; 
					line-height: 1.5;
				}
				.highlight { color: #4f46e5; font-weight: bold; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="success">✅</div>
				<h1>Authentication Successful!</h1>
				<div class="user-info">
					<img src="%s" alt="Avatar" class="avatar">
					<p><strong>Welcome to ZenType, %s!</strong></p>
					<p>GitHub: <span class="highlight">@%s</span></p>
				</div>
				<p><strong>Your Access Token:</strong></p>
				<div class="token" id="token">%s</div>
				<button class="copy-btn" onclick="copyToken()">📋 Copy Token</button>
				<div class="instructions">
					<p>1. Copy the token above</p>
					<p>2. In your terminal, run: <code class="highlight">zentype auth</code></p>
					<p>3. Paste the token when prompted</p>
					<p>4. Start competing: <code class="highlight">zentype start -t 60</code></p>
					<br>
					<p><em>You can now close this window</em></p>
				</div>
			</div>
			<script>
				function copyToken() {
					const token = document.getElementById('token').textContent;
					navigator.clipboard.writeText(token).then(() => {
						const btn = document.querySelector('.copy-btn');
						btn.textContent = '✅ Copied!';
						btn.style.background = '#22c55e';
						setTimeout(() => {
							btn.textContent = '📋 Copy Token';
							btn.style.background = '#4f46e5';
						}, 2000);
					});
				}
				
				// Auto-close after 5 minutes
				setTimeout(() => {
					window.close();
				}, 300000);
			</script>
		</body>
		</html>
	`, githubUser.AvatarURL, username, githubUser.Login, token.AccessToken)
}

func (s *APIServer) verifyToken(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "No token provided", http.StatusUnauthorized)
		return
	}

	// Remove "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")

	var user struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		GitHubID int    `json:"github_id"`
		Login    string `json:"github_login"`
		Avatar   string `json:"avatar_url"`
	}

	err := s.db.QueryRow(`
		SELECT id, username, github_id, github_login, avatar_url 
		FROM users 
		WHERE access_token = $1`,
		token,
	).Scan(&user.ID, &user.Username, &user.GitHubID, &user.Login, &user.Avatar)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (s *APIServer) submitScore(w http.ResponseWriter, r *http.Request) {
	// Verify authentication
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	token = strings.TrimPrefix(token, "Bearer ")

	var userID int
	var username string
	var githubID int
	err := s.db.QueryRow(`
		SELECT id, username, github_id FROM users WHERE access_token = $1`,
		token,
	).Scan(&userID, &username, &githubID)

	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Parse score data
	var entry LeaderboardEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validation
	if entry.Duration != TargetDuration {
		http.Error(w, fmt.Sprintf("Only %d-second tests are supported", TargetDuration), http.StatusBadRequest)
		return
	}

	if entry.WPM < 0 || entry.WPM > 300 {
		http.Error(w, "Invalid WPM value", http.StatusBadRequest)
		return
	}

	if entry.Accuracy < 0 || entry.Accuracy > 100 {
		http.Error(w, "Invalid accuracy value", http.StatusBadRequest)
		return
	}

	if entry.Accuracy < MinAccuracy {
		http.Error(w, fmt.Sprintf("Minimum accuracy of %.1f%% required for leaderboard", MinAccuracy), http.StatusBadRequest)
		return
	}

	// Insert score
	var scoreID int
	var createdAt time.Time
	err = s.db.QueryRow(`
		INSERT INTO scores (user_id, username, github_id, wpm, accuracy, duration, language) 
		VALUES ($1, $2, $3, $4, $5, $6, $7) 
		RETURNING id, created_at`,
		userID, username, githubID, entry.WPM, entry.Accuracy, entry.Duration, entry.Language,
	).Scan(&scoreID, &createdAt)

	if err != nil {
		log.Printf("Error inserting score: %v", err)
		http.Error(w, "Failed to save score", http.StatusInternalServerError)
		return
	}

	// Calculate current rank based on the new score
	var rank int
	err = s.db.QueryRow(`
		WITH user_best_scores AS (
			SELECT 
				github_id,
				CASE 
					WHEN github_id = $4 THEN GREATEST(MAX(wpm), $5)
					ELSE MAX(wpm)
				END as best_wpm,
				CASE 
					WHEN github_id = $4 AND GREATEST(MAX(wpm), $5) = $5 THEN $6
					WHEN github_id = $4 AND GREATEST(MAX(wpm), $5) > $5 THEN MAX(CASE WHEN wpm = MAX(wpm) THEN accuracy END)
					ELSE MAX(CASE WHEN wpm = MAX(wpm) THEN accuracy END)
				END as best_accuracy
			FROM scores 
			WHERE accuracy >= $1 AND duration = $2 AND language = $3
			GROUP BY github_id
		)
		SELECT COUNT(*) + 1
		FROM user_best_scores
		WHERE best_wpm > $5 OR (best_wpm = $5 AND best_accuracy > $6)`,
		MinAccuracy, TargetDuration, entry.Language, githubID, entry.WPM, entry.Accuracy,
	).Scan(&rank)

	if err != nil {
		log.Printf("Error calculating rank: %v", err)
		rank = 0 // Default if rank calculation fails
	}

	// Log the score submission
	log.Printf("✅ Score submitted: %s (%.1f WPM, %.1f%% acc) - Rank #%d", username, entry.WPM, entry.Accuracy, rank)

	// Return response
	response := LeaderboardEntry{
		ID:        scoreID,
		Username:  username,
		GitHubID:  githubID,
		WPM:       entry.WPM,
		Accuracy:  entry.Accuracy,
		Duration:  entry.Duration,
		Language:  entry.Language,
		CreatedAt: createdAt,
		Rank:      rank,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (s *APIServer) getLeaderboard(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("language")
	if language == "" {
		language = "english"
	}

	// Get requesting user's GitHub ID to exclude them
	var requestingUserID int
	token := r.Header.Get("Authorization")
	if token != "" {
		token = strings.TrimPrefix(token, "Bearer ")
		s.db.QueryRow(`SELECT github_id FROM users WHERE access_token = $1`, token).Scan(&requestingUserID)
	}

	// Get top 10 users (best score per user, ties broken by accuracy)
	query := `
		WITH user_best AS (
			SELECT 
				username,
				github_id,
				MAX(wpm) as best_wpm
			FROM scores 
			WHERE accuracy >= $1 AND duration = $2 AND language = $3 AND github_id != $4
			GROUP BY username, github_id
		),
		user_details AS (
			SELECT DISTINCT ON (s.username, s.github_id)
				s.username,
				s.github_id,
				ub.best_wpm,
				s.accuracy as best_accuracy,
				s.created_at as score_date
			FROM scores s
			JOIN user_best ub ON s.username = ub.username AND s.github_id = ub.github_id AND s.wpm = ub.best_wpm
			WHERE s.accuracy >= $1 AND s.duration = $2 AND s.language = $3 AND s.github_id != $4
			ORDER BY s.username, s.github_id, s.accuracy DESC, s.created_at ASC
		)
		SELECT 
			username,
			github_id,
			best_wpm,
			best_accuracy,
			score_date,
			ROW_NUMBER() OVER (ORDER BY best_wpm DESC, best_accuracy DESC, score_date ASC) as rank
		FROM user_details
		ORDER BY rank
		LIMIT 10`

	rows, err := s.db.Query(query, MinAccuracy, TargetDuration, language, requestingUserID)
	if err != nil {
		log.Printf("Error getting leaderboard: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		err := rows.Scan(
			&entry.Username, &entry.GitHubID, &entry.WPM, 
			&entry.Accuracy, &entry.CreatedAt, &entry.Rank,
		)
		if err != nil {
			log.Printf("Error scanning leaderboard row: %v", err)
			continue
		}
		entry.Duration = TargetDuration
		entry.Language = language
		entries = append(entries, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *APIServer) getUserRank(w http.ResponseWriter, r *http.Request) {
	// Verify authentication
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	token = strings.TrimPrefix(token, "Bearer ")

	var githubID int
	var username string
	err := s.db.QueryRow(`
		SELECT github_id, username FROM users WHERE access_token = $1`,
		token,
	).Scan(&githubID, &username)

	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	language := r.URL.Query().Get("language")
	if language == "" {
		language = "english"
	}

	// Get user's best score and rank
	var userStats UserStats
	userStats.Username = username
	userStats.GitHubID = githubID

	// Get user's best score - simplified query
	err = s.db.QueryRow(`
		SELECT 
			COALESCE(MAX(wpm), 0) as best_wpm,
			COUNT(*) as total_scores,
			COUNT(CASE WHEN accuracy >= $1 THEN 1 END) as qualified_scores
		FROM scores 
		WHERE github_id = $2 AND duration = $3 AND language = $4`,
		MinAccuracy, githubID, TargetDuration, language,
	).Scan(&userStats.BestWPM, &userStats.TotalScores, &userStats.QualifiedScores)
	
	// Get best accuracy for the best WPM score
	if userStats.BestWPM > 0 {
		err2 := s.db.QueryRow(`
			SELECT accuracy 
			FROM scores 
			WHERE github_id = $1 AND duration = $2 AND language = $3 AND wpm = $4
			ORDER BY accuracy DESC, created_at ASC
			LIMIT 1`,
			githubID, TargetDuration, language, userStats.BestWPM,
		).Scan(&userStats.BestAccuracy)
		if err2 != nil {
			userStats.BestAccuracy = 0
		}
	}

	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Calculate rank only if user has qualifying scores
	if userStats.QualifiedScores > 0 && userStats.BestWPM > 0 {
		// Simple rank calculation: count users with better scores
		err = s.db.QueryRow(`
			WITH user_best AS (
				SELECT 
					github_id,
					MAX(wpm) as best_wpm,
					MAX(accuracy) as best_accuracy
				FROM scores 
				WHERE accuracy >= $1 AND duration = $2 AND language = $3
				GROUP BY github_id
			)
			SELECT COUNT(*) + 1
			FROM user_best
			WHERE best_wpm > $4 OR (best_wpm = $4 AND best_accuracy > $5)`,
			MinAccuracy, TargetDuration, language, userStats.BestWPM, userStats.BestAccuracy,
		).Scan(&userStats.Rank)

		if err != nil {
			userStats.Rank = 0
		}
	} else {
		userStats.Rank = 0 // Not qualified for leaderboard
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userStats)
}

func (s *APIServer) getGlobalStats(w http.ResponseWriter, r *http.Request) {
	var stats struct {
		TotalUsers      int     `json:"total_users"`
		TotalScores     int     `json:"total_scores"`
		QualifiedScores int     `json:"qualified_scores"`
		HighestWPM      float64 `json:"highest_wpm"`
		AverageWPM      float64 `json:"average_wpm"`
		AverageAccuracy float64 `json:"average_accuracy"`
		TopUser         string  `json:"top_user"`
	}

	// Get basic stats
	err := s.db.QueryRow(`
		SELECT 
			(SELECT COUNT(DISTINCT github_id) FROM scores WHERE accuracy >= $1 AND duration = $2) as total_users,
			(SELECT COUNT(*) FROM scores WHERE accuracy >= $1 AND duration = $2) as qualified_scores,
			(SELECT COUNT(*) FROM scores WHERE duration = $2) as total_scores,
			COALESCE((SELECT MAX(wpm) FROM scores WHERE accuracy >= $1 AND duration = $2), 0) as highest_wpm,
			COALESCE((SELECT AVG(wpm) FROM scores WHERE accuracy >= $1 AND duration = $2), 0) as avg_wpm,
			COALESCE((SELECT AVG(accuracy) FROM scores WHERE accuracy >= $1 AND duration = $2), 0) as avg_accuracy`,
		MinAccuracy, TargetDuration,
	).Scan(&stats.TotalUsers, &stats.QualifiedScores, &stats.TotalScores, 
		&stats.HighestWPM, &stats.AverageWPM, &stats.AverageAccuracy)

	if err != nil {
		log.Printf("Error getting global stats: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Get top user
	err = s.db.QueryRow(`
		SELECT username 
		FROM scores 
		WHERE accuracy >= $1 AND duration = $2 AND wpm = $3
		ORDER BY accuracy DESC, created_at ASC 
		LIMIT 1`,
		MinAccuracy, TargetDuration, stats.HighestWPM,
	).Scan(&stats.TopUser)

	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error getting top user: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
