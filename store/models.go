/* models.go
 * This file contain the interfaces, structs and helper functions that relate to DB objects
 * Authors: Zachary Bower
 */

package store

import (
	"pickems-bot/sources"
	"pickems-bot/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UpcomingMatchDoc represents upcoming match data stored in the database
type UpcomingMatchDoc struct {
	Round            string                    `bson:"round,omitempty"`
	ScheduledMatches []sources.ScheduledMatch `bson:"scheduled_matches,omitempty"`
}

// LeaderboardEntry represents a single entry in the leaderboard for a user
type LeaderboardEntry struct {
	UserID             string `bson:"userid,omitempty"`
	Username           string `bson:"username,omitempty"`
	Score              int    `bson:"score,omitempty"`
	models.ScoreResult `bson:",inline"`
}

// Leaderboard represents the tournament leaderboard stored in MongoDB
type Leaderboard struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Round     string             `bson:"round,omitempty"`
	UpdatedAt time.Time          `bson:"updated_at"`
	Entries   []LeaderboardEntry `bson:"entries"`
}
