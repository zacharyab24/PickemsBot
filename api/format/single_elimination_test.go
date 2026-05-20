/* single_elimination_test.go
 * Tests for the single-elimination format scoring path.
 */

package format

import (
	"testing"

	"pickems-bot/api/external"
	"pickems-bot/api/shared"

	"github.com/stretchr/testify/assert"
)

// TestSingleElimCalculateScore_AllSucceeded tests a fully-correct prediction.
func TestSingleElimCalculateScore_AllSucceeded(t *testing.T) {
	prediction := shared.Prediction{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
		},
	}
	results := EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
		},
	}

	scoreResult, report, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 2, scoreResult.Successes)
	assert.Equal(t, 0, scoreResult.Pending)
	assert.Equal(t, 0, scoreResult.Failed)
	assert.Contains(t, report, "Team A to win the Grand Final")
	assert.Contains(t, report, "Team B to lose in the Semifinal")
}

// TestSingleElimCalculateScore_PendingWhenResultMissing tests a team that hasn't played yet.
func TestSingleElimCalculateScore_PendingWhenResultMissing(t *testing.T) {
	prediction := shared.Prediction{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}
	results := EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team B": {Round: "Quarterfinal", Status: "eliminated"},
		},
	}

	scoreResult, _, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 1, scoreResult.Pending)
}

// TestSingleElimCalculateScore_PendingWhenStatusPending tests an in-progress match.
func TestSingleElimCalculateScore_PendingWhenStatusPending(t *testing.T) {
	prediction := shared.Prediction{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}
	results := EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "pending"},
		},
	}

	scoreResult, _, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 1, scoreResult.Pending)
}

// TestSingleElimCalculateScore_FailedOnMismatch tests a wrong-round prediction.
func TestSingleElimCalculateScore_FailedOnMismatch(t *testing.T) {
	prediction := shared.Prediction{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}
	results := EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team A": {Round: "Quarterfinal", Status: "eliminated"},
		},
	}

	scoreResult, _, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 1, scoreResult.Failed)
}

// TestSingleElimCalculateScore_EmptyPrediction errors when prediction has no entries.
func TestSingleElimCalculateScore_EmptyPrediction(t *testing.T) {
	prediction := shared.Prediction{
		Progression: map[string]shared.TeamProgress{},
	}
	results := EliminationResult{
		Teams: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}

	_, _, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestSingleElimCalculateScore_EmptyResults errors when results map is empty.
func TestSingleElimCalculateScore_EmptyResults(t *testing.T) {
	prediction := shared.Prediction{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}
	results := EliminationResult{
		Teams: map[string]shared.TeamProgress{},
	}

	_, _, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestSingleElimCalculateScore_WrongResultType ensures we error rather than panic.
func TestSingleElimCalculateScore_WrongResultType(t *testing.T) {
	_, _, err := singleElimFormat{}.CalculateScore(shared.Prediction{}, SwissResult{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected EliminationResult")
}

// FromRecord/ToRecord tests removed: those methods no longer exist after the
// in-memory result and storage record types were unified.

// TestSingleElim_RequiredPredictions table-tests the bracket size formula.
func TestSingleElim_RequiredPredictions(t *testing.T) {
	cases := []struct{ teams, want int }{
		{4, 2},
		{8, 4},
		{16, 8},
		{32, 16},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, singleElimFormat{}.RequiredPredictions(c.teams), "teamCount=%d", c.teams)
	}
}

// region getEliminationResults

// TestGetEliminationResults_GrandFinal tests single match (Grand Final)
func TestGetEliminationResults_GrandFinal(t *testing.T) {
	matchNodes := []external.MatchNode{
		{ID: "bracket_R01-M001", Team1: "Winner", Team2: "RunnerUp", Winner: "Winner"},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "Grand Final", results["Winner"].Round)
	assert.Equal(t, "advanced", results["Winner"].Status)
	assert.Equal(t, "Grand Final", results["RunnerUp"].Round)
	assert.Equal(t, "eliminated", results["RunnerUp"].Status)
}

// TestGetEliminationResults_MultipleRounds tests multiple rounds
func TestGetEliminationResults_MultipleRounds(t *testing.T) {
	matchNodes := []external.MatchNode{
		{ID: "bracket_R02-M001", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "bracket_R02-M002", Team1: "TeamC", Team2: "TeamD", Winner: "TeamC"},
		{ID: "bracket_R01-M001", Team1: "TeamA", Team2: "TeamC", Winner: "TeamA"},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Len(t, results, 4)
	assert.Equal(t, "advanced", results["TeamA"].Status)
	assert.Equal(t, "eliminated", results["TeamC"].Status)
	assert.Equal(t, "eliminated", results["TeamB"].Status)
	assert.Equal(t, "eliminated", results["TeamD"].Status)
}

// TestGetEliminationResults_PendingMatches tests with TBD winners
func TestGetEliminationResults_PendingMatches(t *testing.T) {
	matchNodes := []external.MatchNode{
		{ID: "bracket_R01-M001", Team1: "TeamA", Team2: "TeamB", Winner: "TBD"},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "Grand Final", results["TeamA"].Round)
	assert.Equal(t, "pending", results["TeamA"].Status)
	assert.Equal(t, "Grand Final", results["TeamB"].Round)
	assert.Equal(t, "pending", results["TeamB"].Status)
}

// TestGetEliminationResults_EmptyWinner tests with empty winner string
func TestGetEliminationResults_EmptyWinner(t *testing.T) {
	matchNodes := []external.MatchNode{
		{ID: "bracket_R01-M001", Team1: "TeamA", Team2: "TeamB", Winner: ""},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "pending", results["TeamA"].Status)
	assert.Equal(t, "pending", results["TeamB"].Status)
}

// TestGetEliminationResults_EmptyInput tests error with no matches
func TestGetEliminationResults_EmptyInput(t *testing.T) {
	_, err := getEliminationResults([]external.MatchNode{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one match required")
}

// TestGetEliminationResults_EmptyTeamNames tests handling of empty team names
func TestGetEliminationResults_EmptyTeamNames(t *testing.T) {
	matchNodes := []external.MatchNode{
		{ID: "bracket_R01-M001", Team1: "", Team2: "TeamB", Winner: "TeamB"},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Grand Final", results["TeamB"].Round)
	assert.Equal(t, "advanced", results["TeamB"].Status)
}

// endregion

// region getRoundIndex

func TestGetRoundIndex_Found(t *testing.T) {
	rounds := []string{"Grand Final", "Semi Final", "Quarter Final"}
	assert.Equal(t, 2, getRoundIndex("Semi Final", rounds))
}

func TestGetRoundIndex_FirstRound(t *testing.T) {
	rounds := []string{"Grand Final", "Semi Final", "Quarter Final"}
	assert.Equal(t, 3, getRoundIndex("Grand Final", rounds))
}

func TestGetRoundIndex_LastRound(t *testing.T) {
	rounds := []string{"Grand Final", "Semi Final", "Quarter Final"}
	assert.Equal(t, 1, getRoundIndex("Quarter Final", rounds))
}

func TestGetRoundIndex_NotFound(t *testing.T) {
	assert.Equal(t, -1, getRoundIndex("Quarter Final", []string{"Grand Final", "Semi Final"}))
}

func TestGetRoundIndex_EmptyRounds(t *testing.T) {
	assert.Equal(t, -1, getRoundIndex("Grand Final", []string{}))
}

// endregion

// region getRoundNames

func TestGetRoundNames_SingleMatch(t *testing.T) {
	rounds, err := getRoundNames(1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final"}, rounds)
}

func TestGetRoundNames_ThreeMatches(t *testing.T) {
	rounds, err := getRoundNames(3)
	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final", "Semi Final"}, rounds)
}

func TestGetRoundNames_SevenMatches(t *testing.T) {
	rounds, err := getRoundNames(7)
	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final", "Semi Final", "Quarter Final"}, rounds)
}

func TestGetRoundNames_FifteenMatches(t *testing.T) {
	rounds, err := getRoundNames(15)
	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final", "Semi Final", "Quarter Final", "Best of 16"}, rounds)
}

func TestGetRoundNames_ThirtyOneMatches(t *testing.T) {
	rounds, err := getRoundNames(31)
	assert.NoError(t, err)
	assert.Equal(t, []string{"Grand Final", "Semi Final", "Quarter Final", "Best of 16", "Best of 32"}, rounds)
}

func TestGetRoundNames_TooManyMatches(t *testing.T) {
	_, err := getRoundNames(63)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported depth")
}

func TestGetRoundNames_ZeroMatches(t *testing.T) {
	rounds, err := getRoundNames(0)
	assert.NoError(t, err)
	assert.Empty(t, rounds)
}

// endregion

// region extractRoundAndMatchIDs

func TestExtractRoundAndMatchIDs_Valid(t *testing.T) {
	round, match, err := extractRoundAndMatchIDs("RSTxQ88PoQ_R03-M001")
	assert.NoError(t, err)
	assert.Equal(t, 3, round)
	assert.Equal(t, 1, match)
}

func TestExtractRoundAndMatchIDs_DifferentValues(t *testing.T) {
	round, match, err := extractRoundAndMatchIDs("ABC123_R10-M025")
	assert.NoError(t, err)
	assert.Equal(t, 10, round)
	assert.Equal(t, 25, match)
}

func TestExtractRoundAndMatchIDs_InvalidFormat(t *testing.T) {
	_, _, err := extractRoundAndMatchIDs("invalid_format")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ID format")
}

func TestExtractRoundAndMatchIDs_MissingUnderscore(t *testing.T) {
	_, _, err := extractRoundAndMatchIDs("R03-M001")
	assert.Error(t, err)
}

func TestExtractRoundAndMatchIDs_MissingRound(t *testing.T) {
	_, _, err := extractRoundAndMatchIDs("bracket_M001")
	assert.Error(t, err)
}

// endregion

// region ExtractMatchListIDs

// TestSingleElimExtractMatchListIDs_Basic tests Bracket ID extraction
func TestSingleElimExtractMatchListIDs_Basic(t *testing.T) {
	wikitext := `
== Format ==
Single-elimination bracket

{{Bracket|id=BRACKET1}}
`
	ids, kind, err := singleElimFormat{}.ExtractMatchListIDs(wikitext)
	assert.NoError(t, err)
	assert.Equal(t, SingleElim, kind)
	assert.Len(t, ids, 1)
	assert.Contains(t, ids, "BRACKET1")
}

// TestSingleElimExtractMatchListIDs_StripsHTMLComments verifies trailing
// HTML comments are removed from extracted IDs.
func TestSingleElimExtractMatchListIDs_StripsHTMLComments(t *testing.T) {
	wikitext := `
== Format ==
Single-elimination bracket

{{Bracket|id=CLEAN123<!-- some comment -->}}
`
	ids, _, err := singleElimFormat{}.ExtractMatchListIDs(wikitext)
	assert.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, "CLEAN123", ids[0])
}

// endregion
