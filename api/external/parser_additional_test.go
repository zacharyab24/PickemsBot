/* parser_additional_test.go
 * Contains additional unit tests for untested parser.go functions
 * Authors: Zachary Bower
 */

package external

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// region CalculateSwissScores tests

// TestCalculateSwissScores_AllWins tests team with all wins
func TestCalculateSwissScores_AllWins(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "2", Team1: "TeamA", Team2: "TeamC", Winner: "TeamA"},
		{ID: "3", Team1: "TeamA", Team2: "TeamD", Winner: "TeamA"},
	}

	scores, err := CalculateSwissScores(matchNodes)

	assert.NoError(t, err)
	assert.Equal(t, "3-0", scores["TeamA"])
	assert.Equal(t, "0-1", scores["TeamB"])
	assert.Equal(t, "0-1", scores["TeamC"])
	assert.Equal(t, "0-1", scores["TeamD"])
}

// TestCalculateSwissScores_MixedResults tests mixed win/loss records
func TestCalculateSwissScores_MixedResults(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "2", Team1: "TeamA", Team2: "TeamC", Winner: "TeamC"},
		{ID: "3", Team1: "TeamB", Team2: "TeamC", Winner: "TeamB"},
	}

	scores, err := CalculateSwissScores(matchNodes)

	assert.NoError(t, err)
	assert.Equal(t, "1-1", scores["TeamA"])
	assert.Equal(t, "1-1", scores["TeamB"])
	assert.Equal(t, "1-1", scores["TeamC"])
}

// TestCalculateSwissScores_PendingMatches tests with TBD winners (pending matches)
func TestCalculateSwissScores_PendingMatches(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "2", Team1: "TeamA", Team2: "TeamC", Winner: "TBD"}, // Pending
		{ID: "3", Team1: "TeamB", Team2: "TeamC", Winner: "TBD"}, // Pending
	}

	scores, err := CalculateSwissScores(matchNodes)

	assert.NoError(t, err)
	assert.Equal(t, "1-0", scores["TeamA"])
	assert.Equal(t, "0-1", scores["TeamB"])
	assert.Equal(t, "0-0", scores["TeamC"]) // No completed matches
}

// TestCalculateSwissScores_EmptyInput tests with no matches
func TestCalculateSwissScores_EmptyInput(t *testing.T) {
	matchNodes := []MatchNode{}

	scores, err := CalculateSwissScores(matchNodes)

	assert.NoError(t, err)
	assert.Empty(t, scores)
}

// TestCalculateSwissScores_TBDTeamsExcluded tests that TBD teams are excluded
func TestCalculateSwissScores_TBDTeamsExcluded(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TBD", Winner: "TeamA"},
		{ID: "2", Team1: "TBD", Team2: "TeamB", Winner: "TeamB"},
	}

	scores, err := CalculateSwissScores(matchNodes)

	assert.NoError(t, err)
	assert.Equal(t, "1-0", scores["TeamA"])
	assert.Equal(t, "1-0", scores["TeamB"])
	_, hasTBD := scores["TBD"]
	assert.False(t, hasTBD)
}

// TestCalculateSwissScores_InvalidWinner tests handling of unexpected winner values
func TestCalculateSwissScores_InvalidWinner(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamC"}, // Invalid winner
		{ID: "2", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"}, // Valid
	}

	scores, err := CalculateSwissScores(matchNodes)

	assert.NoError(t, err)
	// Invalid match should be skipped
	assert.Equal(t, "1-0", scores["TeamA"])
	assert.Equal(t, "0-1", scores["TeamB"])
}

// endregion

// region GetEliminationResults tests

// TestGetEliminationResults_GrandFinal tests single match (Grand Final)
func TestGetEliminationResults_GrandFinal(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "bracket_R01-M001", Team1: "Winner", Team2: "RunnerUp", Winner: "Winner"},
	}

	results, err := GetEliminationResults(matchNodes)

	assert.NoError(t, err)
	assert.Equal(t, "Grand Final", results["Winner"].Round)
	assert.Equal(t, "advanced", results["Winner"].Status)
	assert.Equal(t, "Grand Final", results["RunnerUp"].Round)
	assert.Equal(t, "eliminated", results["RunnerUp"].Status)
}

// TestGetEliminationResults_MultipleRounds tests multiple rounds
func TestGetEliminationResults_MultipleRounds(t *testing.T) {
	// R01 is Grand Final, R02 is Semi Final (higher round number = earlier round)
	matchNodes := []MatchNode{
		{ID: "bracket_R02-M001", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "bracket_R02-M002", Team1: "TeamC", Team2: "TeamD", Winner: "TeamC"},
		{ID: "bracket_R01-M001", Team1: "TeamA", Team2: "TeamC", Winner: "TeamA"},
	}

	results, err := GetEliminationResults(matchNodes)

	assert.NoError(t, err)
	// Check we have 4 teams
	assert.Len(t, results, 4)

	// TeamA should advance (winner of R01)
	assert.Equal(t, "advanced", results["TeamA"].Status)

	// TeamC should be eliminated in R01 (Grand Final based on round logic)
	assert.Equal(t, "eliminated", results["TeamC"].Status)

	// TeamB and TeamD should be eliminated in R02 (Semi Finals)
	assert.Equal(t, "eliminated", results["TeamB"].Status)
	assert.Equal(t, "eliminated", results["TeamD"].Status)
}

// TestGetEliminationResults_PendingMatches tests with TBD winners
func TestGetEliminationResults_PendingMatches(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "bracket_R01-M001", Team1: "TeamA", Team2: "TeamB", Winner: "TBD"},
	}

	results, err := GetEliminationResults(matchNodes)

	assert.NoError(t, err)
	assert.Equal(t, "Grand Final", results["TeamA"].Round)
	assert.Equal(t, "pending", results["TeamA"].Status)
	assert.Equal(t, "Grand Final", results["TeamB"].Round)
	assert.Equal(t, "pending", results["TeamB"].Status)
}

// TestGetEliminationResults_EmptyWinner tests with empty winner string
func TestGetEliminationResults_EmptyWinner(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "bracket_R01-M001", Team1: "TeamA", Team2: "TeamB", Winner: ""},
	}

	results, err := GetEliminationResults(matchNodes)

	assert.NoError(t, err)
	assert.Equal(t, "pending", results["TeamA"].Status)
	assert.Equal(t, "pending", results["TeamB"].Status)
}

// TestGetEliminationResults_EmptyInput tests error with no matches
func TestGetEliminationResults_EmptyInput(t *testing.T) {
	matchNodes := []MatchNode{}

	_, err := GetEliminationResults(matchNodes)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one match required")
}

// TestGetEliminationResults_EmptyTeamNames tests handling of empty team names
func TestGetEliminationResults_EmptyTeamNames(t *testing.T) {
	matchNodes := []MatchNode{
		{ID: "bracket_R01-M001", Team1: "", Team2: "TeamB", Winner: "TeamB"},
	}

	results, err := GetEliminationResults(matchNodes)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Grand Final", results["TeamB"].Round)
	assert.Equal(t, "advanced", results["TeamB"].Status)
}

// endregion

// region getRoundIndex tests

// TestGetRoundIndex_Found tests finding a round in the list
func TestGetRoundIndex_Found(t *testing.T) {
	rounds := []string{"Grand Final", "Semi Final", "Quarter Final"}

	index := getRoundIndex("Semi Final", rounds)

	assert.Equal(t, 2, index) // len(rounds) - 1 = 3 - 1 = 2
}

// TestGetRoundIndex_FirstRound tests first round
func TestGetRoundIndex_FirstRound(t *testing.T) {
	rounds := []string{"Grand Final", "Semi Final", "Quarter Final"}

	index := getRoundIndex("Grand Final", rounds)

	assert.Equal(t, 3, index) // len(rounds) - 0 = 3
}

// TestGetRoundIndex_LastRound tests last round
func TestGetRoundIndex_LastRound(t *testing.T) {
	rounds := []string{"Grand Final", "Semi Final", "Quarter Final"}

	index := getRoundIndex("Quarter Final", rounds)

	assert.Equal(t, 1, index) // len(rounds) - 2 = 1
}

// TestGetRoundIndex_NotFound tests round not in list
func TestGetRoundIndex_NotFound(t *testing.T) {
	rounds := []string{"Grand Final", "Semi Final"}

	index := getRoundIndex("Quarter Final", rounds)

	assert.Equal(t, -1, index)
}

// TestGetRoundIndex_EmptyRounds tests empty rounds slice
func TestGetRoundIndex_EmptyRounds(t *testing.T) {
	rounds := []string{}

	index := getRoundIndex("Grand Final", rounds)

	assert.Equal(t, -1, index)
}

// endregion

// region getRoundNames tests

// TestGetRoundNames_SingleMatch tests 1 match (Grand Final only)
func TestGetRoundNames_SingleMatch(t *testing.T) {
	rounds, err := getRoundNames(1)

	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final"}, rounds)
}

// TestGetRoundNames_ThreeMatches tests 3 matches (GF + 2 SF)
func TestGetRoundNames_ThreeMatches(t *testing.T) {
	rounds, err := getRoundNames(3)

	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final", "Semi Final"}, rounds)
}

// TestGetRoundNames_SevenMatches tests 7 matches (GF + SF + QF)
func TestGetRoundNames_SevenMatches(t *testing.T) {
	rounds, err := getRoundNames(7)

	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final", "Semi Final", "Quarter Final"}, rounds)
}

// TestGetRoundNames_FifteenMatches tests 15 matches (up to R16)
func TestGetRoundNames_FifteenMatches(t *testing.T) {
	rounds, err := getRoundNames(15)

	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final", "Semi Final", "Quarter Final", "Best of 16"}, rounds)
}

// TestGetRoundNames_ThirtyOneMatches tests 31 matches (full 32-team bracket)
func TestGetRoundNames_ThirtyOneMatches(t *testing.T) {
	rounds, err := getRoundNames(31)

	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final", "Semi Final", "Quarter Final", "Best of 16", "Best of 32"}, rounds)
}

// TestGetRoundNames_TooManyMatches tests error for too many matches
func TestGetRoundNames_TooManyMatches(t *testing.T) {
	_, err := getRoundNames(63) // Would require 6 rounds

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported depth")
}

// TestGetRoundNames_ZeroMatches tests zero matches
func TestGetRoundNames_ZeroMatches(t *testing.T) {
	rounds, err := getRoundNames(0)

	assert.NoError(t, err)
	// log2(0+1) = log2(1) = 0, ceil(0) = 0, so returns empty slice
	assert.Empty(t, rounds)
}

// endregion

// region ExtractRoundAndMatchIDs tests

// TestExtractRoundAndMatchIDs_Valid tests valid ID format
func TestExtractRoundAndMatchIDs_Valid(t *testing.T) {
	round, match, err := ExtractRoundAndMatchIDs("RSTxQ88PoQ_R03-M001")

	assert.NoError(t, err)
	assert.Equal(t, 3, round)
	assert.Equal(t, 1, match)
}

// TestExtractRoundAndMatchIDs_DifferentValues tests different round/match values
func TestExtractRoundAndMatchIDs_DifferentValues(t *testing.T) {
	round, match, err := ExtractRoundAndMatchIDs("ABC123_R10-M025")

	assert.NoError(t, err)
	assert.Equal(t, 10, round)
	assert.Equal(t, 25, match)
}

// TestExtractRoundAndMatchIDs_InvalidFormat tests invalid format
func TestExtractRoundAndMatchIDs_InvalidFormat(t *testing.T) {
	_, _, err := ExtractRoundAndMatchIDs("invalid_format")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ID format")
}

// TestExtractRoundAndMatchIDs_MissingUnderscore tests missing underscore
func TestExtractRoundAndMatchIDs_MissingUnderscore(t *testing.T) {
	_, _, err := ExtractRoundAndMatchIDs("R03-M001")

	assert.Error(t, err)
}

// TestExtractRoundAndMatchIDs_MissingRound tests missing round number
func TestExtractRoundAndMatchIDs_MissingRound(t *testing.T) {
	_, _, err := ExtractRoundAndMatchIDs("bracket_M001")

	assert.Error(t, err)
}

// endregion

// region DetectTournamentFormat tests

// TestDetectTournamentFormat_Swiss tests detecting Swiss format
func TestDetectTournamentFormat_Swiss(t *testing.T) {
	wikitext := `
== Format ==
* The tournament uses a Swiss format
* Teams play best of 3 matches
`

	format := DetectTournamentFormat(wikitext)

	assert.Equal(t, "swiss", format)
}

// TestDetectTournamentFormat_SingleElimination tests detecting single-elimination
func TestDetectTournamentFormat_SingleElimination(t *testing.T) {
	wikitext := `
== Format ==
* Single-elimination bracket
* Best of 5 grand final
`

	format := DetectTournamentFormat(wikitext)

	assert.Equal(t, "single-elimination", format)
}

// TestDetectTournamentFormat_DoubleElimination tests detecting double-elimination
func TestDetectTournamentFormat_DoubleElimination(t *testing.T) {
	wikitext := `
== Format ==
* Double-elimination bracket
* Upper and lower bracket
`

	format := DetectTournamentFormat(wikitext)

	assert.Equal(t, "double-elimination", format)
}

// TestDetectTournamentFormat_BothFormats tests when both Swiss and single-elim mentioned
func TestDetectTournamentFormat_BothFormats(t *testing.T) {
	wikitext := `
== Format ==
* Swiss stage followed by single-elimination playoffs
`

	format := DetectTournamentFormat(wikitext)

	assert.Equal(t, "swiss", format) // Should return swiss per code logic
}

// TestDetectTournamentFormat_CaseInsensitive tests case insensitivity
func TestDetectTournamentFormat_CaseInsensitive(t *testing.T) {
	wikitext := `
== Format ==
* SWISS FORMAT tournament
`

	format := DetectTournamentFormat(wikitext)

	assert.Equal(t, "swiss", format)
}

// TestDetectTournamentFormat_NoFormatSection tests missing format section
func TestDetectTournamentFormat_NoFormatSection(t *testing.T) {
	wikitext := `
== Tournament ==
* Some tournament info
`

	format := DetectTournamentFormat(wikitext)

	assert.Equal(t, "unknown", format)
}

// TestDetectTournamentFormat_UnrecognizedFormat tests unrecognized format
func TestDetectTournamentFormat_UnrecognizedFormat(t *testing.T) {
	wikitext := `
== Format ==
* Round-robin format
`

	format := DetectTournamentFormat(wikitext)

	assert.Equal(t, "unknown", format)
}

// endregion

// region ExtractMatchListID tests

// TestExtractMatchListID_Swiss tests extracting IDs from Swiss format
func TestExtractMatchListID_Swiss(t *testing.T) {
	wikitext := `
== Format ==
Swiss format tournament

{{Matchlist|id=ABC123}}
{{Matchlist|id=DEF456}}
`

	ids, format, err := ExtractMatchListID(wikitext)

	assert.NoError(t, err)
	assert.Equal(t, "swiss", format)
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, "ABC123")
	assert.Contains(t, ids, "DEF456")
}

// TestExtractMatchListID_SingleElimination tests extracting IDs from single-elimination
func TestExtractMatchListID_SingleElimination(t *testing.T) {
	wikitext := `
== Format ==
Single-elimination bracket

{{Bracket|id=BRACKET1}}
`

	ids, format, err := ExtractMatchListID(wikitext)

	assert.NoError(t, err)
	assert.Equal(t, "single-elimination", format)
	assert.Len(t, ids, 1)
	assert.Contains(t, ids, "BRACKET1")
}

// TestExtractMatchListID_NoMatches tests no matching templates
func TestExtractMatchListID_NoMatches(t *testing.T) {
	wikitext := `
== Format ==
Swiss format

Some text but no templates
`

	ids, format, err := ExtractMatchListID(wikitext)

	// Function returns error when no IDs are found
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no ids found")
	assert.Nil(t, ids)
	assert.Empty(t, format)
}

// TestExtractMatchListID_UnknownFormat tests error with unknown format
func TestExtractMatchListID_UnknownFormat(t *testing.T) {
	wikitext := `
== Format ==
Round-robin format

{{Matchlist|id=ABC123}}
`

	_, _, err := ExtractMatchListID(wikitext)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tournament format")
}

// TestExtractMatchListID_WithHTMLComments tests removing HTML comments
func TestExtractMatchListID_WithHTMLComments(t *testing.T) {
	wikitext := `
== Format ==
Single-elimination bracket

{{Bracket|id=CLEAN123<!-- some comment -->}}
`

	ids, format, err := ExtractMatchListID(wikitext)

	assert.NoError(t, err)
	assert.Equal(t, "single-elimination", format)
	assert.Len(t, ids, 1)
	assert.Equal(t, "CLEAN123", ids[0])
}

// endregion
