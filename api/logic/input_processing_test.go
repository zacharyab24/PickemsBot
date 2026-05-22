/* input_processing_test.go
 * Contains unit tests for input_processing.go functions
 * Authors: Zachary Bower
 */

package logic

import (
	"testing"

	"pickems-bot/api/format"
	"pickems-bot/api/shared"

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
	prediction := shared.Prediction{
		Format:  "swiss",
		Win:     []string{"Team A", "Team B", "Team C"},
		Advance: []string{"Team D", "Team E"},
		Lose:    []string{"Team F", "Team G"},
	}

	results := format.SwissResult{
		Teams: map[string]string{
			"Team A": "3-0",
			"Team B": "3-0",
			"Team C": "3-0",
			"Team D": "3-1",
			"Team E": "3-2",
			"Team F": "0-3",
			"Team G": "0-3",
		},
	}

	report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 7, report.GetScore().Successes)
	assert.Equal(t, 0, report.GetScore().Pending)
	assert.Equal(t, 0, report.GetScore().Failed)
	swissReport := report.(format.SwissReport)
	assert.Len(t, swissReport.WinPicks, 3)
	assert.Len(t, swissReport.AdvancePicks, 2)
	assert.Len(t, swissReport.LosePicks, 2)
}

// TestCalculateUserScore_SwissPending tests Swiss score with pending matches
func TestCalculateUserScore_SwissPending(t *testing.T) {
	prediction := shared.Prediction{
		Format:  "swiss",
		Win:     []string{"Team A"},
		Advance: []string{"Team B"},
		Lose:    []string{"Team C"},
	}

	results := format.SwissResult{
		Teams: map[string]string{
			"Team A": "2-0", // Pending (needs 3 wins)
			"Team B": "2-1", // Pending (needs 3 wins)
			"Team C": "0-2", // Pending (needs 3 losses)
		},
	}

	report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, report.GetScore().Successes)
	assert.Equal(t, 3, report.GetScore().Pending)
	assert.Equal(t, 0, report.GetScore().Failed)
}

// TestCalculateUserScore_SwissFailed tests Swiss score with failed predictions
func TestCalculateUserScore_SwissFailed(t *testing.T) {
	prediction := shared.Prediction{
		Format:  "swiss",
		Win:     []string{"Team A"},
		Advance: []string{"Team B"},
		Lose:    []string{"Team C"},
	}

	results := format.SwissResult{
		Teams: map[string]string{
			"Team A": "2-3", // Failed (lost once, should be 3-0)
			"Team B": "0-3", // Failed (went 0-3, should advance)
			"Team C": "3-0", // Failed (won, should go 0-3)
		},
	}

	report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, report.GetScore().Successes)
	assert.Equal(t, 0, report.GetScore().Pending)
	assert.Equal(t, 3, report.GetScore().Failed)
}

// TestCalculateUserScore_SwissMixedResults tests Swiss score with mixed results
func TestCalculateUserScore_SwissMixedResults(t *testing.T) {
	prediction := shared.Prediction{
		Format:  "swiss",
		Win:     []string{"Team A", "Team B"},
		Advance: []string{"Team C", "Team D"},
		Lose:    []string{"Team E", "Team F"},
	}

	results := format.SwissResult{
		Teams: map[string]string{
			"Team A": "3-0", // Success
			"Team B": "2-0", // Pending
			"Team C": "3-1", // Success
			"Team D": "1-0", // Pending
			"Team E": "0-3", // Success
			"Team F": "3-2", // Failed (won when should lose)
		},
	}

	report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, report.GetScore().Successes)
	assert.Equal(t, 2, report.GetScore().Pending)
	assert.Equal(t, 1, report.GetScore().Failed)
}

// TestCalculateUserScore_EliminationSuccess tests elimination bracket scoring with successful predictions
func TestCalculateUserScore_EliminationSuccess(t *testing.T) {
	prediction := shared.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
			"Team C": {Round: "Quarterfinal", Status: "eliminated"},
		},
	}

	results := format.EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
			"Team C": {Round: "Quarterfinal", Status: "eliminated"},
		},
	}

	report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 3, report.GetScore().Successes)
	assert.Equal(t, 0, report.GetScore().Pending)
	assert.Equal(t, 0, report.GetScore().Failed)
	elimReport := report.(format.SingleElimReport)
	var teamA *format.ElimPredictionEntry
	for i := range elimReport.Predictions {
		if elimReport.Predictions[i].Team == "Team A" {
			teamA = &elimReport.Predictions[i]
			break
		}
	}
	assert.NotNil(t, teamA)
	assert.True(t, teamA.ToWin)
	assert.Equal(t, format.StatusSucceeded, teamA.Status)
}

// TestCalculateUserScore_EliminationPending tests elimination with pending results
func TestCalculateUserScore_EliminationPending(t *testing.T) {
	prediction := shared.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
		},
	}

	results := format.EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "pending"},
			"Team B": {Round: "Semifinal", Status: "pending"},
		},
	}

	report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, report.GetScore().Successes)
	assert.Equal(t, 2, report.GetScore().Pending)
	assert.Equal(t, 0, report.GetScore().Failed)
}

// TestCalculateUserScore_EliminationFailed tests elimination with failed predictions
func TestCalculateUserScore_EliminationFailed(t *testing.T) {
	prediction := shared.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
		},
	}

	results := format.EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team A": {Round: "Semifinal", Status: "eliminated"},    // Failed
			"Team B": {Round: "Quarterfinal", Status: "eliminated"}, // Failed
		},
	}

	report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, report.GetScore().Successes)
	assert.Equal(t, 0, report.GetScore().Pending)
	assert.Equal(t, 2, report.GetScore().Failed)
}

// TestCalculateUserScore_EliminationTeamNotInResults tests when predicted team not in results
func TestCalculateUserScore_EliminationTeamNotInResults(t *testing.T) {
	prediction := shared.Prediction{
		Format: "single-elimination",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}

	results := format.EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team B": {Round: "Semifinal", Status: "advanced"}, // Different team, Team A not in results
		},
	}

	report, err := CalculateUserScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, report.GetScore().Successes)
	assert.Equal(t, 1, report.GetScore().Pending) // Missing team counts as pending
	assert.Equal(t, 0, report.GetScore().Failed)
}

// UnknownResult is a mock type for testing unknown result types
type UnknownResult struct{}

func (u UnknownResult) GetType() format.Kind   { return "unknown" }
func (u UnknownResult) GetRound() string       { return "" }
func (u UnknownResult) GetTeamNames() []string { return nil }

// TestCalculateUserScore_UnknownType tests handling of unknown result type
func TestCalculateUserScore_UnknownType(t *testing.T) {
	prediction := shared.Prediction{
		Format: "swiss",
	}

	results := UnknownResult{}

	_, err := CalculateUserScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}
