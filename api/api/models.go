/* models.go
 * This file contain the interfaces, structs and helper functions that are used by api consumers
 * Authors: Zachary Bower
 */

package api

// ScoreResult represents the outcome of score calculation for a user's predictions
type ScoreResult struct {
	Successes int
	Pending   int
	Failed    int
}
