package game

import (
	"strings"
	"time"
)

// TypingStats holds the statistics for a game session
type TypingStats struct {
	WPM               float64
	Accuracy          float64
	CharactersTyped   int
	CorrectChars      int
	TotalChars        int
	TimeElapsed       time.Duration
	IsComplete        bool
	UncorrectedErrors int
}

// TypingGame represents the state of a game session
type TypingGame struct {
	AllWords        []string
	DisplayLines    []string
	UserInput       string
	CurrentPos      int
	GlobalPos       int
	StartTime       time.Time
	Duration        int
	IsStarted       bool
	IsFinished      bool
	Errors          map[int]bool
	TotalErrorsMade int
	LinesPerView    int
	CharsPerLine    int
	WordsTyped      int
}

// NewTypingGame initializes a new TypingGame instance with a specified duration
func NewTypingGame(duration int) *TypingGame {
	// Generate random words from the English word list
	words := GenerateWords(200) // Generate 200 random words for the session
	
	game := &TypingGame{
		AllWords:     words,
		Duration:     duration,
		Errors:       make(map[int]bool),
		LinesPerView: 3,
		CharsPerLine: 50,
	}
	game.generateDisplayLines()
	return game
}

// generateDisplayLines creates the initial display lines based on the words available
func (g *TypingGame) generateDisplayLines() {
	lines := make([]string, 0, g.LinesPerView)
	wordIndex := g.WordsTyped

	// Generate exactly g.LinesPerView lines
	for lineNum := 0; lineNum < g.LinesPerView && wordIndex < len(g.AllWords); lineNum++ {
		var currentLine strings.Builder

		// Fill current line with words
		for wordIndex < len(g.AllWords) {
			word := g.AllWords[wordIndex]
			spaceNeeded := 0
			if currentLine.Len() > 0 {
				spaceNeeded = 1
			}

			// Check if word fits
			if currentLine.Len()+spaceNeeded+len(word) <= g.CharsPerLine {
				if currentLine.Len() > 0 {
					currentLine.WriteString(" ")
				}
				currentLine.WriteString(word)
				wordIndex++
			} else {
				// Word doesn't fit, break to next line
				break
			}
		}

		// Add the completed line
		if currentLine.Len() > 0 {
			lines = append(lines, currentLine.String())
		} else {
			// If no words fit, add empty line
			lines = append(lines, "")
		}
	}

	// Ensure we have exactly g.LinesPerView lines
	for len(lines) < g.LinesPerView {
		lines = append(lines, "")
	}

	g.DisplayLines = lines
}

// Start initializes the game session if it hasn't started yet
func (g *TypingGame) Start() {
	if !g.IsStarted {
		g.StartTime = time.Now()
		g.IsStarted = true
	}
}

// AddCharacter handles user input and updates game state
func (g *TypingGame) AddCharacter(char rune) {
	if !g.IsStarted {
		g.Start()
	}

	if g.IsFinished || g.IsTimeUp() {
		g.IsFinished = true
		return
	}

	lineText := []rune(g.DisplayLines[0])

	// If at end of line, only shift if user just typed space
	if g.CurrentPos == len(lineText) {
		if char == ' ' {
			g.UserInput += string(char)
			g.CurrentPos++
			g.GlobalPos++
			g.shiftLines()
		}
		return
	}

	// Normal character processing
	if g.CurrentPos < len(lineText) && g.CurrentPos >= 0 {
		g.UserInput += string(char)
		if lineText[g.CurrentPos] != char {
			g.Errors[g.GlobalPos] = true
			g.TotalErrorsMade++
		}
		g.CurrentPos++
		g.GlobalPos++
	}
}

// HandleEnterKey handles Enter key press for line progression
func (g *TypingGame) HandleEnterKey() bool {
	if g.IsFinished || g.IsTimeUp() {
		return false
	}

	lineText := []rune(g.DisplayLines[0])

	// Only allow Enter to progress if at end of line
	if g.CurrentPos == len(lineText) {
		// Treat Enter like Space internally for consistency
		g.UserInput += " "
		g.CurrentPos++
		g.GlobalPos++
		g.shiftLines()
		return true
	}

	return false
}

// shiftLines moves to the next line in the game, updating the words typed and generating new lines
func (g *TypingGame) shiftLines() {
	// Move to next line
	g.WordsTyped += len(strings.Fields(g.DisplayLines[0]))
	g.CurrentPos = 0

	// Generate new lines
	g.generateDisplayLines()
	
	// Extend words if we're running low (like in typtea)
	if g.WordsTyped > len(g.AllWords)-50 {
		newWords := GenerateWords(100)
		g.AllWords = append(g.AllWords, newWords...)
	}
}

// RemoveCharacter removes the last character from the user input and updates the position
func (g *TypingGame) RemoveCharacter() {
	if len(g.UserInput) > 0 && g.CurrentPos > 0 {
		g.UserInput = g.UserInput[:len(g.UserInput)-1]
		g.CurrentPos--
		g.GlobalPos--

		// Remove error mark if previously added
		delete(g.Errors, g.GlobalPos)
	}
}

// GetDisplayText returns the current text to be displayed in the game
func (g *TypingGame) GetDisplayText() string {
	return strings.Join(g.DisplayLines, " ")
}

// IsTimeUp checks if the game time has exceeded the specified duration
func (g *TypingGame) IsTimeUp() bool {
	if !g.IsStarted {
		return false
	}
	return time.Since(g.StartTime).Seconds() >= float64(g.Duration)
}

// GetRemainingTime returns the remaining time in seconds for the game
func (g *TypingGame) GetRemainingTime() int {
	if !g.IsStarted {
		return g.Duration
	}
	elapsed := int(time.Since(g.StartTime).Seconds())
	remaining := g.Duration - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetStats calculates and returns the typing statistics for the current game session
func (g *TypingGame) GetStats() TypingStats {
	if !g.IsStarted {
		return TypingStats{}
	}

	elapsed := time.Since(g.StartTime)
	
	// If time is up, use exact test duration for accurate calculations
// This ensures WPM calculation uses the intended time (e.g., exactly 15s)
var timeForCalculation time.Duration
if g.IsTimeUp() {
    timeForCalculation = time.Duration(g.Duration) * time.Second
} else {
    timeForCalculation = elapsed
}
	
	minutes := timeForCalculation.Minutes()

	// Calculate standard WPM (Gross WPM - total characters typed / 5 / minutes)
	wpm := 0.0
	if minutes > 0 {
		wpm = float64(g.GlobalPos) / 5 / minutes
	}

	// Calculate accuracy (correct characters / total characters typed * 100)
	correctChars := g.GlobalPos - g.TotalErrorsMade
	accuracy := 0.0
	if g.GlobalPos > 0 {
		accuracy = float64(correctChars) / float64(g.GlobalPos) * 100
	}

	// Ensure values don't go below 0
	if wpm < 0 {
		wpm = 0  // Fixed the typo here
	}
	if accuracy < 0 {
		accuracy = 0
	}

	return TypingStats{
		WPM:               wpm,  // Use standard WPM, not Net WPM
		Accuracy:          accuracy,
		CharactersTyped:   g.GlobalPos,
		CorrectChars:      correctChars,
		TotalChars:        len([]rune(g.GetDisplayText())),
		TimeElapsed:       timeForCalculation,
		IsComplete:        g.IsFinished,
		UncorrectedErrors: len(g.Errors),
	}
}