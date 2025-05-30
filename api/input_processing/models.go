/* models.go
 * Contains the structs and helper functions used by the input_processing package for storing user predictions in the db
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */

package input_processing

import "go.mongodb.org/mongo-driver/bson/primitive"

type Prediction struct {
	// Generic attributes
	Id primitive.ObjectID  `bson:"_id,omitempty"`
	UserId string `bson:"userid,omitempty"`
	Username string `bson:"username,omitempty"`
	Format string `bson:"format,omitempty"` // "swiss" or "single-elimination"
	Round string `bson:"round,omitempty"`

	// Swiss-specific attributes
	Win []string `bson:"win,omitempty"`
	Advance []string `bson:"advance,omitempty"`
	Lose []string `bson:"lose,omitempty"`

	// Elimination specific attributes
	Progression map[string]TeamProgress `bson:"progression,omitempty"`
}

type TeamProgress struct {
	Round string `bson:"round,omitempty"` // e.g. "semifinal", "grandfinal"
	Status string `bson:"status,omitempty"` // "advanced", "eliminated"
}

type User struct {
	UserId string
	Username string
}

type ScoreResult struct {
	Successes int
	Pending int
	Failed int
}