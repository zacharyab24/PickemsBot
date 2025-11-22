/* models.go
 * This file contain the interfaces, structs and helper functions that are shared between sub packages
 * Authors: Zachary Bower
 */

package shared

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
