/* models.go
 * This file contain the interfaces, structs and helper functions that are shared between sub packages
 * Authors: Zachary Bower
 */

package shared

type User struct {
	UserId   string
	Username string
}

// Team Progress struct
type TeamProgress struct {
	Round  string `bson:"round,omitempty"`  // e.g. "semifinal", "grandfinal"
	Status string `bson:"status,omitempty"` // "advanced", "eliminated"
}