/* storage.go
 * Contains the logic for processing user predictions and leaderboards from a message into something that can be stored in the db
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */

package input_processing

import (
	"github.com/lithammer/fuzzysearch/fuzzy"
)

// Function to process teams names from user input and converts this into a Prediction struct that can be stored in db
// Preconditions: receives a string slice containing user's predicions. In this list the order matters, but we assume it is validated before it is parsed in
// Postconditions: returns the generated Prediction, or an error if it occurs
func setUserPredictions(predictionTeams []string) (Prediction, error)  {
	fuzzy.Match("asdf", "asdfasdf")
	return Prediction{}, nil
}