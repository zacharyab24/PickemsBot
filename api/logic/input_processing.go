/* input_processing.go
 * Contains the logic for processing user input and validating teams
 * Authors: Zachary Bower
 */

package logic

import (
	"strings"

	"pickems-bot/api/format"
	"pickems-bot/api/shared"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// CheckTeamNames processes team names from user input and checks if they are valid.
// Preconditions: Receives two string slices; one containing the user's predictions and another that is a list of valid team names
// Postconditions: Returns two string slices, a slice of correctly formatted team names and slice of strings containing the invalid team names
func CheckTeamNames(predictionTeams []string, validTeams []string) ([]string, []string) {
	var formattedTeamNames []string
	var invalidTeams []string

	// Convert predictionTeams to lowercase for better matching
	lookup := make(map[string]string)
	var validTeamsLower []string
	for _, name := range validTeams {
		lower := strings.ToLower(name)
		lookup[lower] = name
		validTeamsLower = append(validTeamsLower, lower)
	}

	// Match team names
	for _, team := range predictionTeams {
		lowerTeam := strings.ToLower(team)
		fuzzyResults := fuzzy.RankFind(lowerTeam, validTeamsLower)
		// If there is no valid team name, add it to the invalid teams list
		if len(fuzzyResults) == 0 {
			invalidTeams = append(invalidTeams, team)
			continue
		} else if len(fuzzyResults) == 1 {
			formattedTeamNames = append(formattedTeamNames, lookup[fuzzyResults[0].Target]) // Append the original team name, not the lowercase one
		} else if len(fuzzyResults) > 1 { // If there are multiple matches, check to see if theres an exact match with the input
			temp := ""
			for i := range fuzzyResults {
				if fuzzyResults[i].Target == lowerTeam {
					temp = fuzzyResults[i].Target
				}
			}
			// If no exact match was found, take the best ranked match
			if temp == "" {
				temp = fuzzyResults[0].Target
			}
			formattedTeamNames = append(formattedTeamNames, lookup[temp])
		}
	}
	return formattedTeamNames, invalidTeams
}

// CalculateUserScore calculates a user's score based on their predictions by
// dispatching through the format registry — the per-format scoring lives in
// the api/format package.
func CalculateUserScore(userPrediction shared.Prediction, results format.MatchResult) (shared.ScoreResult, string, error) {
	f, err := format.Get(format.Kind(results.GetType()))
	if err != nil {
		return shared.ScoreResult{}, "", err
	}
	return f.CalculateScore(userPrediction, results)
}
