/* models.go
 * This file contain the interfaces, structs and helper functions that are shared between sub packages
 * Authors: Zachary Bower
 */

package shared

import "go.mongodb.org/mongo-driver/bson/primitive"

// User represents a user with their Discord ID and username
type User struct {
	UserID   string
	Username string
}

// TeamProgress represents a team's progress through tournament rounds
type TeamProgress struct {
	Round  string `bson:"round,omitempty"`  // e.g. "semifinal", "grandfinal"
	Status string `bson:"status,omitempty"` // "advanced", "eliminated"
}

// Prediction represents a user's prediction for a tournament round.
// Lives in shared (rather than store) so both the format and store packages
// can reference it without creating an import cycle.
type Prediction struct {
	// Generic attributes
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	UserID   string             `bson:"userid,omitempty"`
	Username string             `bson:"username,omitempty"`
	Format   string             `bson:"format,omitempty"` // "swiss" or "single-elimination"
	Round    string             `bson:"round,omitempty"`

	// Swiss-specific attributes
	Win     []string `bson:"win,omitempty"`
	Advance []string `bson:"advance,omitempty"`
	Lose    []string `bson:"lose,omitempty"`

	// Elimination specific attributes
	Progression map[string]TeamProgress `bson:"progression,omitempty"`
}

// ScoreResult represents the result of scoring a user's predictions.
type ScoreResult struct {
	Successes int `bson:"successes"`
	Pending   int `bson:"pending"`
	Failed    int `bson:"failed"`
}
