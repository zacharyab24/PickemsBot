/* models_test.go
 * Contains unit tests for models.go functions
 * Authors: Zachary Bower
 */

package store

import (
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"testing"

	"github.com/stretchr/testify/assert"
)

// region ToMatchResult tests

// TestToMatchResult_Swiss tests converting Swiss result record
func TestToMatchResult_Swiss(t *testing.T) {
	record := SwissResultRecord{
		Round: "opening-stage",
		TTL:   1234567890,
		Teams: map[string]string{
			"TeamA": "3-0",
			"TeamB": "2-1",
			"TeamC": "0-3",
		},
	}

	result, err := ToMatchResult(record)

	assert.NoError(t, err)
	assert.IsType(t, external.SwissResult{}, result)

	swissResult := result.(external.SwissResult)
	assert.Equal(t, "3-0", swissResult.Scores["TeamA"])
	assert.Equal(t, "2-1", swissResult.Scores["TeamB"])
	assert.Equal(t, "0-3", swissResult.Scores["TeamC"])
}

// TestToMatchResult_Elimination tests converting elimination result record
func TestToMatchResult_Elimination(t *testing.T) {
	record := EliminationResultRecord{
		Round: "playoff-stage",
		TTL:   1234567890,
		Teams: map[string]shared.TeamProgress{
			"Winner":   {Round: "Grand Final", Status: "advanced"},
			"RunnerUp": {Round: "Grand Final", Status: "eliminated"},
			"SemiFinal1": {Round: "Semi Final", Status: "eliminated"},
		},
	}

	result, err := ToMatchResult(record)

	assert.NoError(t, err)
	assert.IsType(t, external.EliminationResult{}, result)

	elimResult := result.(external.EliminationResult)
	assert.Equal(t, "Grand Final", elimResult.Progression["Winner"].Round)
	assert.Equal(t, "advanced", elimResult.Progression["Winner"].Status)
	assert.Equal(t, "Grand Final", elimResult.Progression["RunnerUp"].Round)
	assert.Equal(t, "eliminated", elimResult.Progression["RunnerUp"].Status)
	assert.Equal(t, "Semi Final", elimResult.Progression["SemiFinal1"].Round)
}

// InvalidSwissRecord is a mock type for testing invalid Swiss scores
type InvalidSwissRecord struct{}

func (r InvalidSwissRecord) GetType() string { return "swiss" }
func (r InvalidSwissRecord) GetRound() string { return "test" }
func (r InvalidSwissRecord) GetTTL() int64 { return 123 }
func (r InvalidSwissRecord) GetTeams() map[string]interface{} {
	return map[string]interface{}{
		"TeamA": 123, // Invalid: should be string
	}
}

// TestToMatchResult_Swiss_InvalidScoreType tests error with invalid score type
func TestToMatchResult_Swiss_InvalidScoreType(t *testing.T) {
	record := InvalidSwissRecord{}

	_, err := ToMatchResult(record)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid score for team")
}

// InvalidEliminationRecord is a mock type for testing invalid elimination progression
type InvalidEliminationRecord struct{}

func (r InvalidEliminationRecord) GetType() string { return "single-elimination" }
func (r InvalidEliminationRecord) GetRound() string { return "test" }
func (r InvalidEliminationRecord) GetTTL() int64 { return 123 }
func (r InvalidEliminationRecord) GetTeams() map[string]interface{} {
	return map[string]interface{}{
		"TeamA": "invalid_string", // Should be map[string]interface{}
	}
}

// TestToMatchResult_Elimination_InvalidProgressionType tests error with invalid progression type
func TestToMatchResult_Elimination_InvalidProgressionType(t *testing.T) {
	record := InvalidEliminationRecord{}

	_, err := ToMatchResult(record)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid progression format")
}

// PartialEliminationRecord is a mock type with missing fields
type PartialEliminationRecord struct{}

func (r PartialEliminationRecord) GetType() string { return "single-elimination" }
func (r PartialEliminationRecord) GetRound() string { return "test" }
func (r PartialEliminationRecord) GetTTL() int64 { return 123 }
func (r PartialEliminationRecord) GetTeams() map[string]interface{} {
	return map[string]interface{}{
		"TeamA": map[string]interface{}{
			"round": "Grand Final",
			// Missing status field
		},
	}
}

// TestToMatchResult_Elimination_MissingFields tests handling of missing fields in progression
func TestToMatchResult_Elimination_MissingFields(t *testing.T) {
	record := PartialEliminationRecord{}

	result, err := ToMatchResult(record)

	assert.NoError(t, err)
	elimResult := result.(external.EliminationResult)
	// Should have empty status since it was missing
	assert.Equal(t, "", elimResult.Progression["TeamA"].Status)
	assert.Equal(t, "Grand Final", elimResult.Progression["TeamA"].Round)
}

// UnknownTypeRecord is a mock type for testing unknown result types
type UnknownTypeRecord struct{}

func (r UnknownTypeRecord) GetType() string { return "unknown-format" }
func (r UnknownTypeRecord) GetRound() string { return "test" }
func (r UnknownTypeRecord) GetTTL() int64 { return 123 }
func (r UnknownTypeRecord) GetTeams() map[string]interface{} {
	return map[string]interface{}{}
}

// TestToMatchResult_UnknownType tests error with unknown result type
func TestToMatchResult_UnknownType(t *testing.T) {
	record := UnknownTypeRecord{}

	_, err := ToMatchResult(record)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown result type")
}

// TestToMatchResult_Swiss_EmptyTeams tests converting Swiss with no teams
func TestToMatchResult_Swiss_EmptyTeams(t *testing.T) {
	record := SwissResultRecord{
		Round: "opening-stage",
		TTL:   1234567890,
		Teams: map[string]string{},
	}

	result, err := ToMatchResult(record)

	assert.NoError(t, err)
	swissResult := result.(external.SwissResult)
	assert.Empty(t, swissResult.Scores)
}

// TestToMatchResult_Elimination_EmptyTeams tests converting Elimination with no teams
func TestToMatchResult_Elimination_EmptyTeams(t *testing.T) {
	record := EliminationResultRecord{
		Round: "playoff-stage",
		TTL:   1234567890,
		Teams: map[string]shared.TeamProgress{},
	}

	result, err := ToMatchResult(record)

	assert.NoError(t, err)
	elimResult := result.(external.EliminationResult)
	assert.Empty(t, elimResult.Progression)
}

// endregion

// region Interface method tests

// TestSwissResultRecord_GetType tests SwissResultRecord.GetType
func TestSwissResultRecord_GetType(t *testing.T) {
	record := SwissResultRecord{}
	assert.Equal(t, "swiss", record.GetType())
}

// TestSwissResultRecord_GetRound tests SwissResultRecord.GetRound
func TestSwissResultRecord_GetRound(t *testing.T) {
	record := SwissResultRecord{Round: "test-round"}
	assert.Equal(t, "test-round", record.GetRound())
}

// TestSwissResultRecord_GetTTL tests SwissResultRecord.GetTTL
func TestSwissResultRecord_GetTTL(t *testing.T) {
	record := SwissResultRecord{TTL: 9876543210}
	assert.Equal(t, int64(9876543210), record.GetTTL())
}

// TestSwissResultRecord_GetTeams tests SwissResultRecord.GetTeams
func TestSwissResultRecord_GetTeams(t *testing.T) {
	record := SwissResultRecord{
		Teams: map[string]string{
			"Team1": "3-0",
			"Team2": "2-1",
		},
	}

	teams := record.GetTeams()
	assert.Len(t, teams, 2)
	assert.Equal(t, "3-0", teams["Team1"])
	assert.Equal(t, "2-1", teams["Team2"])
}

// TestEliminationResultRecord_GetType tests EliminationResultRecord.GetType
func TestEliminationResultRecord_GetType(t *testing.T) {
	record := EliminationResultRecord{}
	assert.Equal(t, "single-elimination", record.GetType())
}

// TestEliminationResultRecord_GetRound tests EliminationResultRecord.GetRound
func TestEliminationResultRecord_GetRound(t *testing.T) {
	record := EliminationResultRecord{Round: "playoff-round"}
	assert.Equal(t, "playoff-round", record.GetRound())
}

// TestEliminationResultRecord_GetTTL tests EliminationResultRecord.GetTTL
func TestEliminationResultRecord_GetTTL(t *testing.T) {
	record := EliminationResultRecord{TTL: 1111111111}
	assert.Equal(t, int64(1111111111), record.GetTTL())
}

// TestEliminationResultRecord_GetTeams tests EliminationResultRecord.GetTeams
func TestEliminationResultRecord_GetTeams(t *testing.T) {
	record := EliminationResultRecord{
		Teams: map[string]shared.TeamProgress{
			"Team1": {Round: "Grand Final", Status: "advanced"},
			"Team2": {Round: "Semi Final", Status: "eliminated"},
		},
	}

	teams := record.GetTeams()
	assert.Len(t, teams, 2)

	// Verify Team1
	team1Data := teams["Team1"].(map[string]interface{})
	assert.Equal(t, "Grand Final", team1Data["round"])
	assert.Equal(t, "advanced", team1Data["status"])

	// Verify Team2
	team2Data := teams["Team2"].(map[string]interface{})
	assert.Equal(t, "Semi Final", team2Data["round"])
	assert.Equal(t, "eliminated", team2Data["status"])
}

// endregion
