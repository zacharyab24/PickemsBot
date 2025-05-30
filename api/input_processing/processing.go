/* processing.go
 * Contains the logic for processing user predictions and leaderboards from a message into something that can be stored in the db
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */

package input_processing

import (
	"fmt"
	"pickems-bot/api/match_data"
	"slices"
	"strconv"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// Function to process teams names from user input and check if they are valid
// Preconditions: receives two string slices; one containing the user's predictions and another that is a list of valid team names
// Postconditions: returns two string slices, a slice of correctly formatted team names and slice of strings containing the invalid team names
func CheckTeamNames(predictionTeams []string, validTeams []string) ([]string, []string)  {
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
		fuzzyResults := fuzzy.Find(lowerTeam, validTeamsLower)
		if len(fuzzyResults) == 1 {
			formattedTeamNames = append(formattedTeamNames, lookup[fuzzyResults[0]]) // Append the original team name, not the lowercase one	
		} else {
			invalidTeams = append(invalidTeams, team)
		}
	}
	return formattedTeamNames, invalidTeams
}

// Function to generate user prediction object to be stored in db
// Precondtions: Receives a user struct, strings containing round and format, string slice of teams, and required number of teams
// teams []string assumes the following: team names are valid, they are ordered correctly. The correct order for this is: 
// swiss: teams[0..1]: win, teams[2..7]: advance and teams[8..9]: lose
// single elim: teams[0]: gf winner, teams[1]: gf loser, teams[2..3]: sf loser, teams[4..7]: qf loser, teams[8-15]: b16 loser, teams[16-31]: b32 loser
// Postconditions: Returns a Prediction that is ready to be inserted into the db, or returns an error that occurs
func GeneratePrediction(user User, format string, round string, teams []string, numRequiredTeams int) (Prediction, error) {
	// Check if slice of teams provided by the user has the correct amount of teams 
	if numRequiredTeams != len(teams) {
		return Prediction{}, fmt.Errorf("this tournament requires %d teams but input was %d", numRequiredTeams, len(teams))
	}

	// Set generic attributes for Prediction struct
	prediction := Prediction {
		UserId: user.UserId,
		Username: user.Username,
		Format: format,
		Round: round,
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
func setSwissPredictions(teams []string) ([]string, []string, []string){
	win := teams[0:2]
	advance := teams[2:8]
	lose := teams[8:10]

	return win, advance, lose
}

// Helper function to generate teamName : TeamProgress map used in single-elim only attributes of Prediction struct
// Precondtions: Receives string slice containing the team names. We assume they arrive in the correct order and the right number
// Postcondtions: Returns map of team name: TeamProgress struct
func setEliminationPredictions(teams []string) map[string]TeamProgress {
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

	progression := make(map[string]TeamProgress)

	// Base case, we have to do this outside the loop because log(0) is undefined and this lets us easily set status as advanced not eliminated
	progression[teams[0]] = TeamProgress{roundNames[pointer], "advanced"}

	threshold := 1
	count := 0
	
	for i := 1; i <= len(teams)-1; i++ {
		// Add team to progression map
		progression[teams[i]] = TeamProgress{roundNames[pointer], "eliminated"}
		
		// If we reach our threshold for how many teams are in this round, we need to increment the roundName pointer and update threshold
		count ++
		if count == threshold {
			pointer++
			threshold *= 2
			count = 0
		}
		
	}
	return progression
}

// Function to calculate a user's score
// Preconditons: Receives a Prediction struct with the data to check, and a match_data.ResultRecord which serves as the point of truth to check against
// Postconditions: Returns a ScoreResult containing the number of successes, fails and pending results or an error if it occurs
func CalculateUserScore(userPrediction Prediction, results match_data.MatchResult) (ScoreResult, error) {
	switch r := results.(type) {
	case match_data.SwissResult:
		scores, err := calculateSwissScore(userPrediction, r.Scores)
		if err != nil {
			return ScoreResult{}, err
		}
		return scores, nil

	case match_data.EliminationResult:
		scores, err := calculateEliminationScore(userPrediction, r.Progression)
		if err != nil {
			return ScoreResult{}, err
		}
		return scores, nil
	default:
		return ScoreResult{}, fmt.Errorf("unknown type: %s", r.GetType())
	}
}

// Function to caluclate a user's score for a swiss tournament
// Preconditions: Receives a Prediction and a teams map with the structre: teamName : score
// Postconditions: Returns a ScoreResult containing the number of successes, failures and pending results, or an error if it occurs
func calculateSwissScore(prediction Prediction, teams map[string]string) (ScoreResult, error) {
	// 3-0 logic
	threeZeroFn := func(wins, loses int) string {
		if loses >= 1 {
			return "Failed"
		} else if wins != 3 {
			return "Pending"
		}
		return "Succeeded"
	}

	// Advance logic
	advanceFn := func(wins, loses int) string {
		if loses == 3 || (wins == 3 && loses == 0) {
			return "Failed"
		} else if wins < 3 {
			return "Pending"
		}
		return "Succeeded"
	}

	// 0-3 logic
	zeroThreeFn := func(wins, loses int) string {
		if wins >= 1 {
			return "Failed"
		} else if loses != 3 {
			return "Pending"
		}
		return "Succeeded"
	}

	var succeeded, pending, failed int
	var s, p, f int
	var err error

	// Calculate win score
	s, p, f, err = evaluateSwissPrediction(prediction.Win, teams, threeZeroFn)
	if err != nil {
		return ScoreResult{}, err
	}
	succeeded += s
	pending += p
	failed += f

	// Calculate advance score
	s, p, f, err = evaluateSwissPrediction(prediction.Advance, teams, advanceFn)
	if err != nil {
		return ScoreResult{}, err
	}
	succeeded += s
	pending += p
	failed += f

	// Calculate lose score
	s, p, f, err = evaluateSwissPrediction(prediction.Lose, teams, zeroThreeFn)
	if err != nil {
		return ScoreResult{}, err
	}
	succeeded += s
	pending += p
	failed += f

	return ScoreResult{succeeded, pending, failed}, nil
}

// Helper function to parse a swiss score string (<wins>-<loses>). This function is limited to single digits for wins and loses
// However the rest of the logic follows a 16 team swiss tournament, which will have at most 5 games played which is well below our limit
// Preconditions: receives string of the form <wins>-<loses>
// Returns 2 ints: wins, loses, or an error if it occurs
func parseSwissScore(score string) (int, int, error) {
	if len(score) != 3 || score[1] != '-' {
		return 0, 0, fmt.Errorf("invalid score format, expected <wins>-<loses> but got %s", score)
	}
	wins, err := strconv.Atoi(string(score[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid wins in score, expected int but got %s", score[0])
	}
	loses, err := strconv.Atoi(string(score[2]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid loses in score, expected int but got %s", score[0])
	}
	return wins, loses, nil
}

// Helper function to get the number of successful wins, advance and lose predictions for a user
// Preconditions: Receives string slice with team names, results map[string]string (team: score), and evaluation function (threeZero, advance or zeroThree) 
// Postconditions: Returns integers containing number of successes, pending results, and failed in that order, or an error if it occurs
func evaluateSwissPrediction(teams []string, r map[string]string, evalFn func(wins, loses int) string) (succeeded, pending, failed int, err error) {
	for _, team := range teams {
		score, ok := r[team]
		if !ok {
			return 0, 0, 0, fmt.Errorf("team score not found for %s", team)
		}
		wins, loses, err := parseSwissScore(score)
		if err != nil {
			return 0, 0, 0, err
		}
		result := evalFn(wins, loses)
		switch result {
		case "Succeeded":
			succeeded++
		case "Pending":
			pending++
		case "Failed":
			failed++
		}
	}
	return
}

// Function to calcluate a user's score for a single-elimination tournament
// Preconditions: Receives a Prediction and a teams map with the structre: teamName : teamProgress
// Postconditions: Returns a ScoreResult containing the number of successes, failures and pending results, or an error if it occurs
func calculateEliminationScore(prediction Prediction, results map[string]match_data.TeamProgress) (ScoreResult, error) {
	var succeeded, pending, failed int
	
	if len(prediction.Progression) == 0 || len(results) == 0 {
		return ScoreResult{}, fmt.Errorf("prediction progress or results progress cannot be empty")
	}

	for team, predictedProgress := range prediction.Progression {
		resultProgress, ok := results[team]
		if !ok {
			// Predicted team is not in result team map
			failed ++
			continue
		}
		
		// Check if prediction was correct
		if predictedProgress.Round == resultProgress.Round {
			if predictedProgress.Status == resultProgress.Status {
				succeeded++
			} else if resultProgress.Status == "pending" {
				pending++
			}
		} else {
			failed++
		}
	}
		
	return ScoreResult{succeeded, pending, failed}, nil
}