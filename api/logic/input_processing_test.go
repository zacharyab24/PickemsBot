/* input_processing_test.go
 * Contains unit tests for input_processing.go functions
 * Authors: Zachary Bower
 */

package logic

import (
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCheckTeamNames_ExactMatches tests exact team name matching
func TestCheckTeamNames_ExactMatches(t *testing.T) {
	validTeams := []string{"Team A", "Team B", "Team C"}
	predictionTeams := []string{"Team A", "Team B", "Team C"}

	formatted, invalid := CheckTeamNames(predictionTeams, validTeams)

	assert.Equal(t, []string{"Team A", "Team B", "Team C"}, formatted)
	assert.Empty(t, invalid)
}

// TestCheckTeamNames_CaseInsensitive tests case-insensitive matching
func TestCheckTeamNames_CaseInsensitive(t *testing.T) {
	validTeams := []string{"FaZe Clan", "Natus Vincere", "G2 Esports"}
	predictionTeams := []string{"faze clan", "NATUS VINCERE", "g2 EsPoRtS"}

	formatted, invalid := CheckTeamNames(predictionTeams, validTeams)

	assert.Equal(t, []string{"FaZe Clan", "Natus Vincere", "G2 Esports"}, formatted)
	assert.Empty(t, invalid)
}

// TestCheckTeamNames_FuzzyMatching tests fuzzy matching for typos
func TestCheckTeamNames_FuzzyMatching(t *testing.T) {
	validTeams := []string{"FaZe Clan", "Natus Vincere", "G2 Esports"}
	predictionTeams := []string{"FaZe", "Natus", "G2"}

	formatted, invalid := CheckTeamNames(predictionTeams, validTeams)

	assert.Len(t, formatted, 3)
	assert.Empty(t, invalid)
}

// TestCheckTeamNames_InvalidTeams tests handling of invalid team names
func TestCheckTeamNames_InvalidTeams(t *testing.T) {
	validTeams := []string{"Team A", "Team B", "Team C"}
	predictionTeams := []string{"Team A", "InvalidTeam", "Team B", "AnotherInvalid"}

	formatted, invalid := CheckTeamNames(predictionTeams, validTeams)

	assert.Equal(t, []string{"Team A", "Team B"}, formatted)
	assert.Equal(t, []string{"InvalidTeam", "AnotherInvalid"}, invalid)
}

// TestCheckTeamNames_MultipleMatches tests handling when fuzzy matching finds multiple results
func TestCheckTeamNames_MultipleMatches(t *testing.T) {
	validTeams := []string{"Cloud9", "Cloud9 Blue", "Cloud9 White"}
	predictionTeams := []string{"Cloud9"}

	formatted, invalid := CheckTeamNames(predictionTeams, validTeams)

	// Should return the exact match or best ranked match
	assert.Len(t, formatted, 1)
	assert.Contains(t, formatted, "Cloud9")
	assert.Empty(t, invalid)
}

// TestCheckTeamNames_EmptyInput tests behavior with empty inputs
func TestCheckTeamNames_EmptyInput(t *testing.T) {
	validTeams := []string{"Team A", "Team B"}
	predictionTeams := []string{}

	formatted, invalid := CheckTeamNames(predictionTeams, validTeams)

	assert.Empty(t, formatted)
	assert.Empty(t, invalid)
}

// TestCheckTeamNames_AllInvalid tests when all teams are invalid
func TestCheckTeamNames_AllInvalid(t *testing.T) {
	validTeams := []string{"Team A", "Team B"}
	predictionTeams := []string{"XYZ", "ABC", "DEF"}

	formatted, invalid := CheckTeamNames(predictionTeams, validTeams)

	assert.Empty(t, formatted)
	assert.Len(t, invalid, 3)
}

// TestCalculateUserScore_SwissSuccess tests Swiss score calculation with successful predictions
func TestCalculateUserScore_SwissSuccess(t *testing.T) {
	prediction := store.Prediction{
		Format:  "swiss",
		Win:     []string{"Team A", "Team B", "Team C"},
		Advance: []string{"Team D", "Team E"},
		Lose:    []string{"Team F", "Team G"},
	}

	results := external.SwissResult{
		Scores: map[string]string{
			"Team A": "3-0",
			"Team B": "3-0",
			"Team C": "3-0",
			"Team D": "3-1",
			"Team E": "3-2",
			"Team F": "0-3",
			"Team G": "0-3",
		},
	}

	scoreResult, report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 7, scoreResult.Successes)
	assert.Equal(t, 0, scoreResult.Pending)
	assert.Equal(t, 0, scoreResult.Failed)
	assert.Contains(t, report, "[3-0]")
	assert.Contains(t, report, "[3-1, 3-2]")
	assert.Contains(t, report, "[0-3]")
}

// TestCalculateUserScore_SwissPending tests Swiss score with pending matches
func TestCalculateUserScore_SwissPending(t *testing.T) {
	prediction := store.Prediction{
		Format:  "swiss",
		Win:     []string{"Team A"},
		Advance: []string{"Team B"},
		Lose:    []string{"Team C"},
	}

	results := external.SwissResult{
		Scores: map[string]string{
			"Team A": "2-0", // Pending (needs 3 wins)
			"Team B": "2-1", // Pending (needs 3 wins)
			"Team C": "0-2", // Pending (needs 3 losses)
		},
	}

	scoreResult, report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 3, scoreResult.Pending)
	assert.Equal(t, 0, scoreResult.Failed)
	assert.Contains(t, report, "[Pending]")
}

// TestCalculateUserScore_SwissFailed tests Swiss score with failed predictions
func TestCalculateUserScore_SwissFailed(t *testing.T) {
	prediction := store.Prediction{
		Format:  "swiss",
		Win:     []string{"Team A"},
		Advance: []string{"Team B"},
		Lose:    []string{"Team C"},
	}

	results := external.SwissResult{
		Scores: map[string]string{
			"Team A": "2-3", // Failed (lost once, should be 3-0)
			"Team B": "0-3", // Failed (went 0-3, should advance)
			"Team C": "3-0", // Failed (won, should go 0-3)
		},
	}

	scoreResult, report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 0, scoreResult.Pending)
	assert.Equal(t, 3, scoreResult.Failed)
	assert.Contains(t, report, "[Failed]")
}

// TestCalculateUserScore_SwissMixedResults tests Swiss score with mixed results
func TestCalculateUserScore_SwissMixedResults(t *testing.T) {
	prediction := store.Prediction{
		Format:  "swiss",
		Win:     []string{"Team A", "Team B"},
		Advance: []string{"Team C", "Team D"},
		Lose:    []string{"Team E", "Team F"},
	}

	results := external.SwissResult{
		Scores: map[string]string{
			"Team A": "3-0", // Success
			"Team B": "2-0", // Pending
			"Team C": "3-1", // Success
			"Team D": "2-1", // Pending
			"Team E": "0-3", // Success
			"Team F": "3-2", // Failed (won when should lose)
		},
	}

	scoreResult, _, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scoreResult.Successes)
	assert.Equal(t, 2, scoreResult.Pending)
	assert.Equal(t, 1, scoreResult.Failed)
}

// TestCalculateUserScore_EliminationSuccess tests elimination bracket scoring with successful predictions
func TestCalculateUserScore_EliminationSuccess(t *testing.T) {
	prediction := store.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
			"Team C": {Round: "Quarterfinal", Status: "eliminated"},
		},
	}

	results := external.EliminationResult{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
			"Team C": {Round: "Quarterfinal", Status: "eliminated"},
		},
	}

	scoreResult, report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, scoreResult.Successes)
	assert.Equal(t, 0, scoreResult.Pending)
	assert.Equal(t, 0, scoreResult.Failed)
	assert.Contains(t, report, "Team A to win the Grand Final")
	assert.Contains(t, report, "[Succeeded]")
}

// TestCalculateUserScore_EliminationPending tests elimination with pending results
func TestCalculateUserScore_EliminationPending(t *testing.T) {
	prediction := store.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
		},
	}

	results := external.EliminationResult{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "pending"},
			"Team B": {Round: "Semifinal", Status: "pending"},
		},
	}

	scoreResult, report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 2, scoreResult.Pending)
	assert.Equal(t, 0, scoreResult.Failed)
	assert.Contains(t, report, "[Pending]")
}

// TestCalculateUserScore_EliminationFailed tests elimination with failed predictions
func TestCalculateUserScore_EliminationFailed(t *testing.T) {
	prediction := store.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
		},
	}

	results := external.EliminationResult{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Semifinal", Status: "eliminated"},    // Failed
			"Team B": {Round: "Quarterfinal", Status: "eliminated"}, // Failed
		},
	}

	scoreResult, report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 0, scoreResult.Pending)
	assert.Equal(t, 2, scoreResult.Failed)
	assert.Contains(t, report, "[Failed]")
}

// TestCalculateUserScore_EliminationTeamNotInResults tests when predicted team not in results
func TestCalculateUserScore_EliminationTeamNotInResults(t *testing.T) {
	prediction := store.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}

	results := external.EliminationResult{
		Progression: map[string]shared.TeamProgress{
			"Team B": {Round: "Semifinal", Status: "advanced"}, // Different team, Team A not in results
		},
	}

	scoreResult, report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 1, scoreResult.Pending) // Missing team counts as pending
	assert.Equal(t, 0, scoreResult.Failed)
	assert.Contains(t, report, "[Pending]")
}

// UnknownResult is a mock type for testing unknown result types
type UnknownResult struct{}

func (u UnknownResult) GetType() string { return "unknown" }

// TestCalculateUserScore_UnknownType tests handling of unknown result type
func TestCalculateUserScore_UnknownType(t *testing.T) {
	prediction := store.Prediction{
		Format: "swiss",
	}

	results := UnknownResult{}

	_, _, err := CalculateUserScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type")
}

// TestCalculateSwissScore_MissingTeam tests when a predicted team has no score in results
func TestCalculateSwissScore_MissingTeam(t *testing.T) {
	prediction := store.Prediction{
		Win:     []string{"Team A"},
		Advance: []string{},
		Lose:    []string{},
	}

	scores := map[string]string{
		// Team A is missing
	}

	scoreResult, report, err := calculateSwissScore(prediction, scores)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 0, scoreResult.Pending)
	assert.Equal(t, 1, scoreResult.Failed)
	assert.Contains(t, report, "Missing score")
}

// TestCalculateSwissScore_InvalidScoreFormat tests handling of invalid score formats
func TestCalculateSwissScore_InvalidScoreFormat(t *testing.T) {
	prediction := store.Prediction{
		Win:     []string{"Team A"},
		Advance: []string{},
		Lose:    []string{},
	}

	scores := map[string]string{
		"Team A": "invalid", // Invalid format
	}

	_, _, err := calculateSwissScore(prediction, scores)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid score format")
}

// TestCalculateSwissScore_InvalidScoreNumbers tests handling of non-numeric scores
func TestCalculateSwissScore_InvalidScoreNumbers(t *testing.T) {
	prediction := store.Prediction{
		Win:     []string{"Team A"},
		Advance: []string{},
		Lose:    []string{},
	}

	scores := map[string]string{
		"Team A": "a-b", // Non-numeric
	}

	_, _, err := calculateSwissScore(prediction, scores)

	assert.Error(t, err)
}

// TestCalculateSwissScore_AdvanceCategory3_0 tests that 3-0 teams fail advance category
func TestCalculateSwissScore_AdvanceCategory3_0(t *testing.T) {
	prediction := store.Prediction{
		Win:     []string{},
		Advance: []string{"Team A"},
		Lose:    []string{},
	}

	scores := map[string]string{
		"Team A": "3-0", // Should fail advance (belongs in win category)
	}

	scoreResult, _, err := calculateSwissScore(prediction, scores)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 1, scoreResult.Failed)
}

// TestCalculateSwissScore_AdvanceCategory0_3 tests that 0-3 teams fail advance category
func TestCalculateSwissScore_AdvanceCategory0_3(t *testing.T) {
	prediction := store.Prediction{
		Win:     []string{},
		Advance: []string{"Team A"},
		Lose:    []string{},
	}

	scores := map[string]string{
		"Team A": "0-3", // Should fail advance
	}

	scoreResult, _, err := calculateSwissScore(prediction, scores)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 1, scoreResult.Failed)
}

// TestCalculateEliminationScore_EmptyPrediction tests error when prediction is empty
func TestCalculateEliminationScore_EmptyPrediction(t *testing.T) {
	prediction := store.Prediction{
		Format:      "single-elimination",
		Progression: map[string]shared.TeamProgress{},
	}

	results := external.EliminationResult{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}

	_, _, err := CalculateUserScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestCalculateEliminationScore_EmptyResults tests error when results are empty
func TestCalculateEliminationScore_EmptyResults(t *testing.T) {
	prediction := store.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}

	results := external.EliminationResult{
		Progression: map[string]shared.TeamProgress{},
	}

	_, _, err := CalculateUserScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestEvaluateSwissPrediction_AllScenarios tests the helper function with all evaluation paths
func TestEvaluateSwissPrediction_AllScenarios(t *testing.T) {
	var builder strings.Builder
	teams := []string{"Team A", "Team B", "Team C"}
	scores := map[string]string{
		"Team A": "3-0",
		"Team B": "2-0",
		"Team C": "1-2",
	}

	// Test Win evaluation (3-0 only)
	evalFn := func(wins, loses int) string {
		if loses >= 1 {
			return "[Failed]"
		} else if wins != 3 {
			return "[Pending]"
		}
		return "[Succeeded]"
	}

	scoreResult, err := evaluateSwissPrediction(teams, scores, evalFn, &builder)

	assert.NoError(t, err)
	assert.Equal(t, 1, scoreResult.Successes) // Team A
	assert.Equal(t, 1, scoreResult.Pending)   // Team B
	assert.Equal(t, 1, scoreResult.Failed)    // Team C
}
