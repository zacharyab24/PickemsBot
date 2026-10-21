/* models.go
 * This file contain the interfaces, structs and helper functions that are shared between sub packages
 * Authors: Zachary Bower
 */

package models

// User represents a user with their Discord ID and username
type User struct {
	UserID   string
	Username string
}

// TeamProgress represents a team's progress through tournament rounds
type TeamProgress struct {
	Round  string // e.g. "semifinal", "grandfinal"
	Status string // "advanced", "eliminated"
}

// Prediction represents a user's prediction for a tournament round.
// Lives in shared (rather than store) so both the format and store packages
// can reference it without creating an import cycle.
type Prediction struct {
	// Generic attributes
	UserID   string
	Username string
	GuildID  string
	Format   string // "swiss" or "single-elimination"
	Round    string

	// Swiss-specific attributes
	Win     []string
	Advance []string
	Lose    []string

	// Elimination specific attributes
	Progression map[string]TeamProgress
}

// PredictionField is a format-agnostic name/value pair used to display a
// prediction summary (e.g. in a Discord embed).
type PredictionField struct {
	Name  string
	Value string
}

// ScoreResult represents the result of scoring a user's predictions.
type ScoreResult struct {
	Successes int
	Pending   int
	Failed    int
}
