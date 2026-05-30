/* input_processing.go
 * Contains the logic for processing user input and validating teams
 * Authors: Zachary Bower
 */

package scoring

import (
	"strings"

	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/tournament"

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
// the tournament package.
// Team names in the prediction are resolved against the result's team names before
// scoring, handling mismatches between data sources (e.g. "FaZe Clan" vs "FaZe").
func CalculateUserScore(userPrediction models.Prediction, results tournament.MatchResult) (tournament.ScoreReport, error) {
	f, err := tournament.Get(tournament.Kind(results.GetType()))
	if err != nil {
		return nil, err
	}
	userPrediction = resolveNamesInPrediction(userPrediction, results.GetTeamNames())
	return f.CalculateScore(userPrediction, results)
}

// resolveNamesInPrediction normalises prediction team names to match the canonical
// names in the result, handling cross-source mismatches. Normalisation runs first;
// fuzzy matching is the fallback for cases normalisation cannot resolve (e.g. abbreviations).
// The original stored prediction is never modified.
func resolveNamesInPrediction(p models.Prediction, validTeams []string) models.Prediction {
	normToOriginal := make(map[string]string, len(validTeams))
	normKeys := make([]string, 0, len(validTeams))
	for _, vt := range validTeams {
		k := sources.NormalizeTeamName(vt)
		normToOriginal[k] = vt
		normKeys = append(normKeys, k)
	}

	resolve := func(name string) string {
		norm := sources.NormalizeTeamName(name)
		if original, ok := normToOriginal[norm]; ok {
			return original
		}
		if matches := fuzzy.RankFind(norm, normKeys); len(matches) > 0 {
			return normToOriginal[matches[0].Target]
		}
		return name
	}

	resolveSlice := func(names []string) []string {
		out := make([]string, len(names))
		for i, n := range names {
			out[i] = resolve(n)
		}
		return out
	}

	p.Win = resolveSlice(p.Win)
	p.Advance = resolveSlice(p.Advance)
	p.Lose = resolveSlice(p.Lose)

	if len(p.Progression) > 0 {
		resolved := make(map[string]models.TeamProgress, len(p.Progression))
		for name, progress := range p.Progression {
			resolved[resolve(name)] = progress
		}
		p.Progression = resolved
	}

	return p
}
