/* prediction.go
 * Contains the logic for generating and processing predictions
 * Authors: Zachary Bower
 */

package logic

import (
	"fmt"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
	"slices"
)

// Function to generate user prediction object to be stored in db
// Precondtions: Receives a user struct, strings containing round and format, string slice of teams, and required number of teams
// teams []string assumes the following: team names are valid, they are ordered correctly. The correct order for this is:
// swiss: teams[0..1]: win, teams[2..7]: advance and teams[8..9]: lose
// single elim: teams[0]: gf winner, teams[1]: gf loser, teams[2..3]: sf loser, teams[4..7]: qf loser, teams[8-15]: b16 loser, teams[16-31]: b32 loser
// Postconditions: Returns a Prediction that is ready to be inserted into the db, or returns an error that occurs
func GeneratePrediction(user shared.User, format string, round string, teams []string, numRequiredTeams int) (store.Prediction, error) {
	// Check if slice of teams provided by the user has the correct amount of teams
	if numRequiredTeams != len(teams) {
		return store.Prediction{}, fmt.Errorf("this tournament requires %d teams but input was %d", numRequiredTeams, len(teams))
	}

	// Set generic attributes for Prediction struct
	prediction := store.Prediction{
		UserID:   user.UserID,
		Username: user.Username,
		Format:   format,
		Round:    round,
	}

	// Set format specific attributes using helper functions
	switch format {
	case "swiss":
		win, advance, lose := setSwissPredictions(teams)
		prediction.Win = win
		prediction.Advance = advance
		prediction.Lose = lose
	case "single-elimination":
		progression := setEliminationPredictions(teams)
		prediction.Progression = progression
	}

	return prediction, nil
}

// Helper function to break up teams slice into 3 slices for swiss-only attributes of Prediction struct
// Preconditions: Receives string slice with exactly 10 valid team names
// Postconditions: Returns 3 string slices (win, advance, lose) according to format defined above
func setSwissPredictions(teams []string) ([]string, []string, []string) {
	win := teams[0:2]
	advance := teams[2:8]
	lose := teams[8:10]

	return win, advance, lose
}

// Helper function to generate teamName : TeamProgress map used in single-elim only attributes of Prediction struct
// Precondtions: Receives string slice containing the team names. We assume they arrive in the correct order and the right number
// Postcondtions: Returns map of team name: TeamProgress struct
func setEliminationPredictions(teams []string) map[string]shared.TeamProgress {
	// Hard coded list of round names. We are limited to single elim brackets of size 32 due to other constraints in the project
	roundNames := []string{
		"Grand Final",
		"Semi Final",
		"Quarter Final",
		"Best of 16",
		"Best of 32",
	}
	pointer := 0

	// Input is from lowest to highest i.e. B32 -> B16 -> QF -> SF -> GF, if we reverse it the logic is a lot simpler
	slices.Reverse(teams)

	progression := make(map[string]shared.TeamProgress)

	// Base case, we have to do this outside the loop because log(0) is undefined and this lets us easily set status as advanced not eliminated
	progression[teams[0]] = shared.TeamProgress{Round: roundNames[pointer], Status: "advanced"}

	threshold := 1
	count := 0

	for i := 1; i <= len(teams)-1; i++ {
		// Add team to progression map
		progression[teams[i]] = shared.TeamProgress{Round: roundNames[pointer], Status: "eliminated"}

		// If we reach our threshold for how many teams are in this round, we need to increment the roundName pointer and update threshold
		count++
		if count == threshold {
			pointer++
			threshold *= 2
			count = 0
		}

	}
	return progression
}
