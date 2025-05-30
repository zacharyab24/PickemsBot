/* models.go
 * This file contain the interfaces, structs and helper functions that are used by api consumers
 * Authors: Zachary Bower
 */

package api

type ScoreResult struct {
	Successes int
	Pending   int
	Failed    int
}