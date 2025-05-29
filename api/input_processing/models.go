/* models.go
 * Contains the structs and helper functions used by the input_processing package for storing user predictions in the db
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */

package processor

import "go.mongodb.org/mongo-driver/bson/primitive"

type Prediction struct {
	// Generic attributes
	Id primitive.ObjectID  `bson:"_id,omitempty"`
	UserId string `bson:"userid,omitempty"`
	Username string `bson:"username,ommitempty"`
	Format string `bson:"format,ommitempty"` // "swiss" or "single-elimination"

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