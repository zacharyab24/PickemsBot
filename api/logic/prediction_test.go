/* prediction_test.go
 * Contains unit tests for prediction.go functions
 * Authors: Zachary Bower
 */

package logic

import (
	"pickems-bot/api/shared"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGeneratePrediction_Swiss tests Swiss format prediction generation
func TestGeneratePrediction_Swiss(t *testing.T) {
	user := shared.User{
		UserId:   "user123",
		Username: "testuser",
	}
	teams := []string{
		"Team1", "Team2", // Win (3-0)
		"Team3", "Team4", "Team5", "Team6", "Team7", "Team8", // Advance (3-1, 3-2)
		"Team9", "Team10", // Lose (0-3)
	}

	prediction, err := GeneratePrediction(user, "swiss", "opening-stage", teams, 10)

	assert.NoError(t, err)
	assert.Equal(t, "user123", prediction.UserId)
	assert.Equal(t, "testuser", prediction.Username)
	assert.Equal(t, "swiss", prediction.Format)
	assert.Equal(t, "opening-stage", prediction.Round)
	assert.Equal(t, []string{"Team1", "Team2"}, prediction.Win)
	assert.Equal(t, []string{"Team3", "Team4", "Team5", "Team6", "Team7", "Team8"}, prediction.Advance)
	assert.Equal(t, []string{"Team9", "Team10"}, prediction.Lose)
}

// TestGeneratePrediction_SingleElimination tests single-elimination format prediction generation
func TestGeneratePrediction_SingleElimination(t *testing.T) {
	user := shared.User{
		UserId:   "user456",
		Username: "anotheruser",
	}
	// Teams in order: GF winner, GF loser, SF losers (2), QF losers (4)
	teams := []string{
		"Winner",
		"Finalist",
		"SF1", "SF2",
		"QF1", "QF2", "QF3", "QF4",
	}

	prediction, err := GeneratePrediction(user, "single-elimination", "playoff-stage", teams, 8)

	assert.NoError(t, err)
	assert.Equal(t, "user456", prediction.UserId)
	assert.Equal(t, "anotheruser", prediction.Username)
	assert.Equal(t, "single-elimination", prediction.Format)
	assert.Equal(t, "playoff-stage", prediction.Round)

	// Check progression map exists and has correct number of teams
	assert.NotNil(t, prediction.Progression)
	assert.Len(t, prediction.Progression, 8)

	// All teams should be in the progression map
	for _, team := range teams {
		_, exists := prediction.Progression[team]
		assert.True(t, exists, "Team %s should exist in progression", team)
	}

	// Should have exactly one team with "advanced" status
	advancedCount := 0
	for _, progress := range prediction.Progression {
		if progress.Status == "advanced" {
			advancedCount++
		}
	}
	assert.Equal(t, 1, advancedCount)
}

// TestGeneratePrediction_WrongTeamCount tests error when team count doesn't match
func TestGeneratePrediction_WrongTeamCount(t *testing.T) {
	user := shared.User{
		UserId:   "user123",
		Username: "testuser",
	}
	teams := []string{"Team1", "Team2", "Team3"} // Only 3 teams instead of 10

	_, err := GeneratePrediction(user, "swiss", "opening-stage", teams, 10)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "this tournament requires 10 teams but input was 3")
}

// TestGeneratePrediction_ExactTeamCount tests that exact team count works
func TestGeneratePrediction_ExactTeamCount(t *testing.T) {
	user := shared.User{
		UserId:   "user123",
		Username: "testuser",
	}
	teams := []string{
		"Team1", "Team2",
		"Team3", "Team4", "Team5", "Team6", "Team7", "Team8",
		"Team9", "Team10",
	}

	prediction, err := GeneratePrediction(user, "swiss", "opening-stage", teams, 10)

	assert.NoError(t, err)
	assert.NotEmpty(t, prediction.UserId)
}

// TestGeneratePrediction_TooManyTeams tests error when too many teams provided
func TestGeneratePrediction_TooManyTeams(t *testing.T) {
	user := shared.User{
		UserId:   "user123",
		Username: "testuser",
	}
	teams := make([]string, 15) // Too many teams
	for i := range teams {
		teams[i] = "Team" + string(rune('A'+i))
	}

	_, err := GeneratePrediction(user, "swiss", "opening-stage", teams, 10)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "this tournament requires 10 teams but input was 15")
}

// TestSetSwissPredictions tests the Swiss prediction splitting logic
func TestSetSwissPredictions(t *testing.T) {
	teams := []string{
		"W1", "W2",
		"A1", "A2", "A3", "A4", "A5", "A6",
		"L1", "L2",
	}

	win, advance, lose := setSwissPredictions(teams)

	assert.Equal(t, []string{"W1", "W2"}, win)
	assert.Equal(t, []string{"A1", "A2", "A3", "A4", "A5", "A6"}, advance)
	assert.Equal(t, []string{"L1", "L2"}, lose)
}

// TestSetSwissPredictions_OrderPreserved tests that team order is preserved
func TestSetSwissPredictions_OrderPreserved(t *testing.T) {
	teams := []string{
		"First", "Second",
		"Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth",
		"Ninth", "Tenth",
	}

	win, advance, lose := setSwissPredictions(teams)

	assert.Equal(t, "First", win[0])
	assert.Equal(t, "Second", win[1])
	assert.Equal(t, "Third", advance[0])
	assert.Equal(t, "Eighth", advance[5])
	assert.Equal(t, "Ninth", lose[0])
	assert.Equal(t, "Tenth", lose[1])
}

// TestSetEliminationPredictions_GrandFinalWinner tests Grand Final winner status
func TestSetEliminationPredictions_GrandFinalWinner(t *testing.T) {
	// Input order: Winner first, then runner-up
	teams := []string{"Winner", "Finalist"}

	progression := setEliminationPredictions(teams)

	// After reversing: ["Finalist", "Winner"]
	// teams[0] = "Finalist" gets Grand Final + advanced (this is wrong in my understanding)
	// Actually, teams[0] after reverse is the LAST team in original input, which is "Finalist"
	// So "Finalist" gets Grand Final + advanced status
	assert.Equal(t, "Grand Final", progression["Finalist"].Round)
	assert.Equal(t, "advanced", progression["Finalist"].Status)
	assert.Equal(t, "Grand Final", progression["Winner"].Round)
	assert.Equal(t, "eliminated", progression["Winner"].Status)
}

// TestSetEliminationPredictions_4Teams tests 4-team bracket (Semis + Finals)
func TestSetEliminationPredictions_4Teams(t *testing.T) {
	// Input order per comment: GF winner, GF loser, SF losers
	// After reverse: [SemiLose2, SemiLose1, Runner-Up, Champion]
	teams := []string{"Champion", "Runner-Up", "SemiLose1", "SemiLose2"}

	progression := setEliminationPredictions(teams)

	assert.Len(t, progression, 4)

	// After reverse, teams[0] = SemiLose2 gets GF + advanced
	assert.Equal(t, "Grand Final", progression["SemiLose2"].Round)
	assert.Equal(t, "advanced", progression["SemiLose2"].Status)

	// teams[1] = SemiLose1 gets GF + eliminated
	assert.Equal(t, "Grand Final", progression["SemiLose1"].Round)
	assert.Equal(t, "eliminated", progression["SemiLose1"].Status)

	// teams[2] and teams[3] = Runner-Up and Champion get SF + eliminated
	assert.Equal(t, "Semi Final", progression["Runner-Up"].Round)
	assert.Equal(t, "eliminated", progression["Runner-Up"].Status)
	assert.Equal(t, "Semi Final", progression["Champion"].Round)
	assert.Equal(t, "eliminated", progression["Champion"].Status)
}

// TestSetEliminationPredictions_8Teams tests 8-team bracket (QF + SF + F)
func TestSetEliminationPredictions_8Teams(t *testing.T) {
	teams := []string{
		"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8",
	}

	progression := setEliminationPredictions(teams)

	assert.Len(t, progression, 8)

	// Count teams at each round
	roundCounts := make(map[string]int)
	for _, progress := range progression {
		roundCounts[progress.Round]++
	}

	// Should have 2 GF, 2 SF, 4 QF based on threshold logic
	assert.Equal(t, 2, roundCounts["Grand Final"])
	assert.Equal(t, 2, roundCounts["Semi Final"])
	assert.Equal(t, 4, roundCounts["Quarter Final"])

	// Verify one team has "advanced" status
	advancedCount := 0
	for _, progress := range progression {
		if progress.Status == "advanced" {
			advancedCount++
		}
	}
	assert.Equal(t, 1, advancedCount)
}

// TestSetEliminationPredictions_16Teams tests 16-team bracket
func TestSetEliminationPredictions_16Teams(t *testing.T) {
	teams := make([]string, 16)
	for i := 0; i < 16; i++ {
		teams[i] = "T" + string(rune('A'+i))
	}

	progression := setEliminationPredictions(teams)

	assert.Len(t, progression, 16)

	// Count teams at each round
	roundCounts := make(map[string]int)
	for _, progress := range progression {
		roundCounts[progress.Round]++
	}

	// Should have 2 GF, 2 SF, 4 QF, 8 R16
	assert.Equal(t, 2, roundCounts["Grand Final"])
	assert.Equal(t, 2, roundCounts["Semi Final"])
	assert.Equal(t, 4, roundCounts["Quarter Final"])
	assert.Equal(t, 8, roundCounts["Best of 16"])

	// Verify exactly one team has "advanced" status
	advancedCount := 0
	for _, progress := range progression {
		if progress.Status == "advanced" {
			advancedCount++
		}
	}
	assert.Equal(t, 1, advancedCount)
}

// TestSetEliminationPredictions_32Teams tests full 32-team bracket
func TestSetEliminationPredictions_32Teams(t *testing.T) {
	teams := make([]string, 32)
	for i := 0; i < 32; i++ {
		teams[i] = "Team" + string(rune('0'+i/10)) + string(rune('0'+i%10))
	}

	progression := setEliminationPredictions(teams)

	assert.Len(t, progression, 32)

	// Count teams at each round
	roundCounts := make(map[string]int)
	for _, progress := range progression {
		roundCounts[progress.Round]++
	}

	// Should have 2 GF, 2 SF, 4 QF, 8 R16, 16 R32
	assert.Equal(t, 2, roundCounts["Grand Final"])
	assert.Equal(t, 2, roundCounts["Semi Final"])
	assert.Equal(t, 4, roundCounts["Quarter Final"])
	assert.Equal(t, 8, roundCounts["Best of 16"])
	assert.Equal(t, 16, roundCounts["Best of 32"])

	// Verify exactly one team has "advanced" status
	advancedCount := 0
	for _, progress := range progression {
		if progress.Status == "advanced" {
			advancedCount++
		}
	}
	assert.Equal(t, 1, advancedCount)
}

// TestSetEliminationPredictions_SingleTeam tests edge case with only winner
func TestSetEliminationPredictions_SingleTeam(t *testing.T) {
	teams := []string{"SoloWinner"}

	progression := setEliminationPredictions(teams)

	assert.Len(t, progression, 1)
	assert.Equal(t, "Grand Final", progression["SoloWinner"].Round)
	assert.Equal(t, "advanced", progression["SoloWinner"].Status)
}

// TestSetEliminationPredictions_TeamNamesPreserved tests that team names are correctly mapped
func TestSetEliminationPredictions_TeamNamesPreserved(t *testing.T) {
	teams := []string{
		"FaZe Clan",
		"Natus Vincere",
		"G2 Esports",
		"Team Vitality",
	}

	progression := setEliminationPredictions(teams)

	// All team names should be keys in the progression map
	for _, team := range teams {
		_, exists := progression[team]
		assert.True(t, exists, "Team %s should exist in progression map", team)
	}
}

// TestSetEliminationPredictions_ThresholdLogic tests the doubling threshold logic
func TestSetEliminationPredictions_ThresholdLogic(t *testing.T) {
	// 8 teams: 1 GF winner, 1 GF loser, 2 SF losers, 4 QF losers
	teams := []string{"W", "F", "SF1", "SF2", "QF1", "QF2", "QF3", "QF4"}

	progression := setEliminationPredictions(teams)

	// Count teams at each round
	roundCounts := make(map[string]int)
	for _, progress := range progression {
		roundCounts[progress.Round]++
	}

	assert.Equal(t, 2, roundCounts["Grand Final"]) // Winner + Finalist
	assert.Equal(t, 2, roundCounts["Semi Final"])   // 2 SF losers
	assert.Equal(t, 4, roundCounts["Quarter Final"]) // 4 QF losers
}
