/* input_processing.go
 * Contains the logic for processing user input and validating teams
 * Authors: Zachary Bower
 */

package logic

import (
	"fmt"
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
	"strconv"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// CheckTeamNames processes team names from user input and checks if they are valid.
// Preconditions: receives two string slices; one containing the user's predictions and another that is a list of valid team names
// Postconditions: returns two string slices, a slice of correctly formatted team names and slice of strings containing the invalid team names
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

// Function to calculate a user's score
// Preconditons: Receives a Prediction struct with the data to check, and a match_data.ResultRecord which serves as the point of truth to check against
// Postconditions: Returns a ScoreResult containing the number of successes, fails and pending results or an error if it occurs
func CalculateUserScore(userPrediction store.Prediction, results external.MatchResult) (store.ScoreResult, string, error) {
	switch r := results.(type) {
	case external.SwissResult:
		scores, report, err := calculateSwissScore(userPrediction, r.Scores)
		if err != nil {
			return store.ScoreResult{}, "", err
		}
		return scores, report, nil

	case external.EliminationResult:
		scores, report, err := calculateEliminationScore(userPrediction, r.Progression)
		if err != nil {
			return store.ScoreResult{}, "", err
		}
		return scores, report, nil
	default:
		return store.ScoreResult{}, "", fmt.Errorf("unknown type: %s", r.GetType())
	}
}

// Function to caluclate a user's score for a swiss tournament
// Preconditions: Receives a Prediction and a teams map with the structre: teamName : score
// Postconditions: Returns a ScoreResult containing the number of successes, failures and pending results, or an error if it occurs
func calculateSwissScore(prediction store.Prediction, teams map[string]string) (store.ScoreResult, string, error) {
	var succeeded, pending, failed int
	var response strings.Builder

	// [3-0]
	response.WriteString("[3-0]\n")
	scoreResult, err := evaluateSwissPrediction(prediction.Win, teams, func(wins, loses int) string {
		if loses >= 1 {
			return "[Failed]"
		} else if wins != 3 {
			return "[Pending]"
		}
		return "[Succeeded]"
	}, &response)
	if err != nil {
		return store.ScoreResult{}, "", err
	}
	succeeded += scoreResult.Successes
	pending += scoreResult.Pending
	failed += scoreResult.Failed

	// [3-1, 3-2]
	response.WriteString("[3-1, 3-2]\n")
	scoreResult, err = evaluateSwissPrediction(prediction.Advance, teams, func(wins, loses int) string {
		if loses == 3 || (wins == 3 && loses == 0) {
			return "[Failed]"
		} else if wins < 3 {
			return "[Pending]"
		}
		return "[Succeeded]"
	}, &response)
	if err != nil {
		return store.ScoreResult{}, "", err
	}
	succeeded += scoreResult.Successes
	pending += scoreResult.Pending
	failed += scoreResult.Failed

	// [0-3]
	response.WriteString("[0-3]\n")
	scoreResult, err = evaluateSwissPrediction(prediction.Lose, teams, func(wins, loses int) string {
		if wins >= 1 {
			return "[Failed]"
		} else if loses != 3 {
			return "[Pending]"
		}
		return "[Succeeded]"
	}, &response)
	if err != nil {
		return store.ScoreResult{}, "", err
	}
	succeeded += scoreResult.Successes
	pending += scoreResult.Pending
	failed += scoreResult.Failed

	return store.ScoreResult{
		Successes: succeeded,
		Pending:   pending,
		Failed:    failed,
	}, response.String(), nil
}

// Helper function to get the number of successful wins, advance and lose predictions for a user
// Preconditions: Receives string slice with team names, results map[string]string (team: score), and evaluation function (threeZero, advance or zeroThree) and a builder function for the string
// Postconditions: Returns a ScoreResult containing the number of successes, failures and pending results, or an error if it occurs
func evaluateSwissPrediction(teams []string, scores map[string]string, evalFn func(wins, loses int) string, builder *strings.Builder) (store.ScoreResult, error) {
	var succeeded, pending, failed int

	for _, team := range teams {
		score, ok := scores[team]
		if !ok {
			builder.WriteString(fmt.Sprintf("%s: [Missing score] [Failed]\n", team))
			failed++
			continue
		}

		if len(score) != 3 || score[1] != '-' {
			return store.ScoreResult{}, fmt.Errorf("invalid score format: %s", score)
		}

		wins, err := strconv.Atoi(string(score[0]))
		if err != nil {
			return store.ScoreResult{}, err
		}
		loses, err := strconv.Atoi(string(score[2]))
		if err != nil {
			return store.ScoreResult{}, err
		}

		result := evalFn(wins, loses)
		builder.WriteString(fmt.Sprintf("%s: %s %s\n", team, score, result))

		switch result {
		case "[Succeeded]":
			succeeded++
		case "[Pending]":
			pending++
		case "[Failed]":
			failed++
		}
	}
	return store.ScoreResult{Successes: succeeded, Pending: pending, Failed: failed}, nil
}

// Function to calcluate a user's score for a single-elimination tournament
// Preconditions: Receives a Prediction and a teams map with the structre: teamName : teamProgress
// Postconditions: Returns a ScoreResult containing the number of successes, failures and pending results, or an error if it occurs
func calculateEliminationScore(prediction store.Prediction, results map[string]shared.TeamProgress) (store.ScoreResult, string, error) {
	var succeeded, pending, failed int
	var response strings.Builder

	if len(prediction.Progression) == 0 || len(results) == 0 {
		return store.ScoreResult{}, "", fmt.Errorf("prediction progress or results progress cannot be empty")
	}

	for team, predictedProgress := range prediction.Progression {
		resultProgress, ok := results[team]

		if predictedProgress.Round == "Grand Final" && predictedProgress.Status == "advanced" {
			response.WriteString(fmt.Sprintf("- %s to win the %s", team, predictedProgress.Round))
		} else {
			response.WriteString(fmt.Sprintf("- %s to make it to the %s", team, predictedProgress.Round))
		}

		// Check if the  prediction was correct
		if !ok || resultProgress.Status == "pending" {
			response.WriteString(" [Pending]\n")
			pending++
		} else if predictedProgress.Round == resultProgress.Round && predictedProgress.Status == resultProgress.Status {
			response.WriteString(" [Succeeded]\n")
			succeeded++
		} else {
			response.WriteString(" [Failed]\n")
			failed++
		}
	}

	return store.ScoreResult{
		Successes: succeeded,
		Pending:   pending,
		Failed:    failed,
	}, response.String(), nil
}
