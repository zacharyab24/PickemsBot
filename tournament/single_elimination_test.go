/* single_elimination_test.go
 * Tests for the single-elimination format scoring path.
 */

package tournament

import (
	"testing"

	"pickems-bot/sources"
	"pickems-bot/models"

	"github.com/stretchr/testify/assert"
)

// TestSingleElimCalculateScore_AllSucceeded tests a fully-correct prediction.
func TestSingleElimCalculateScore_AllSucceeded(t *testing.T) {
	prediction := models.Prediction{
		Progression: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
		},
	}
	results := EliminationResult{
		Teams: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Semifinal", Status: "eliminated"},
		},
	}

	report, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 2, report.GetScore().Successes)
	assert.Equal(t, 0, report.GetScore().Pending)
	assert.Equal(t, 0, report.GetScore().Failed)

	r := report.(SingleElimReport)
	var foundA, foundB bool
	for _, p := range r.Predictions {
		if p.Team == "Team A" && p.ToWin && p.Round == "Grand Final" {
			foundA = true
		}
		if p.Team == "Team B" && !p.ToWin && p.Round == "Semifinal" {
			foundB = true
		}
	}
	assert.True(t, foundA, "expected Team A predicted as Grand Final winner")
	assert.True(t, foundB, "expected Team B predicted as Semifinal loser")
}

// TestSingleElimCalculateScore_PendingWhenResultMissing tests a team that hasn't played yet.
func TestSingleElimCalculateScore_PendingWhenResultMissing(t *testing.T) {
	prediction := models.Prediction{
		Progression: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}
	results := EliminationResult{
		Teams: map[string]models.TeamProgress{
			"Team B": {Round: "Quarterfinal", Status: "eliminated"},
		},
	}

	report, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 1, report.GetScore().Pending)
}

// TestSingleElimCalculateScore_PendingWhenStatusPending tests an in-progress match.
func TestSingleElimCalculateScore_PendingWhenStatusPending(t *testing.T) {
	prediction := models.Prediction{
		Progression: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}
	results := EliminationResult{
		Teams: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "pending"},
		},
	}

	report, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 1, report.GetScore().Pending)
}

// TestSingleElimCalculateScore_FailedOnMismatch tests a wrong-round prediction.
func TestSingleElimCalculateScore_FailedOnMismatch(t *testing.T) {
	prediction := models.Prediction{
		Progression: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}
	results := EliminationResult{
		Teams: map[string]models.TeamProgress{
			"Team A": {Round: "Quarterfinal", Status: "eliminated"},
		},
	}

	report, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 1, report.GetScore().Failed)
}

// TestSingleElimCalculateScore_EmptyPrediction errors when prediction has no entries.
func TestSingleElimCalculateScore_EmptyPrediction(t *testing.T) {
	prediction := models.Prediction{
		Progression: map[string]models.TeamProgress{},
	}
	results := EliminationResult{
		Teams: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}

	_, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestSingleElimCalculateScore_EmptyResults errors when results map is empty.
func TestSingleElimCalculateScore_EmptyResults(t *testing.T) {
	prediction := models.Prediction{
		Progression: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
		},
	}
	results := EliminationResult{
		Teams: map[string]models.TeamProgress{},
	}

	_, err := singleElimFormat{}.CalculateScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestSingleElimCalculateScore_WrongResultType ensures we error rather than panic.
func TestSingleElimCalculateScore_WrongResultType(t *testing.T) {
	_, err := singleElimFormat{}.CalculateScore(models.Prediction{}, SwissResult{})

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
	matchNodes := []sources.MatchNode{
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
	matchNodes := []sources.MatchNode{
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
	matchNodes := []sources.MatchNode{
		{ID: "bracket_R01-M001", Team1: "TeamA", Team2: "TeamB", Winner: "TBD"},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "Grand Final", results["TeamA"].Round)
	assert.Equal(t, "pending", results["TeamA"].Status)
	assert.Equal(t, "Grand Final", results["TeamB"].Round)
	assert.Equal(t, "pending", results["TeamB"].Status)
}

// region modern ID format (position-based fallback)

// TestGetEliminationResults_ModernID_GrandFinal verifies position-based fallback
// for a single-match bracket (Grand Final only) with a modern opaque bracket ID.
func TestGetEliminationResults_ModernID_GrandFinal(t *testing.T) {
	matchNodes := []sources.MatchNode{
		{ID: "Ffcq6omMBC_RxMTP", Team1: "Winner", Team2: "RunnerUp", Winner: "Winner"},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "Grand Final", results["Winner"].Round)
	assert.Equal(t, "advanced", results["Winner"].Status)
	assert.Equal(t, "Grand Final", results["RunnerUp"].Round)
	assert.Equal(t, "eliminated", results["RunnerUp"].Status)
}

// TestGetEliminationResults_ModernID_Bracket8 simulates a full Bracket/8 (QF→SF→Final)
// with modern opaque IDs. Matches are in bracket order as LiquipediaDB returns them.
func TestGetEliminationResults_ModernID_Bracket8(t *testing.T) {
	// 4 QF → 2 SF → 1 Final; winner chain: A>B, C>D, E>F, G>H → A>C, E>G → A>E
	matchNodes := []sources.MatchNode{
		{ID: "BracketId_QF1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"}, // QF pos 0
		{ID: "BracketId_QF2", Team1: "TeamC", Team2: "TeamD", Winner: "TeamC"}, // QF pos 1
		{ID: "BracketId_QF3", Team1: "TeamE", Team2: "TeamF", Winner: "TeamE"}, // QF pos 2
		{ID: "BracketId_QF4", Team1: "TeamG", Team2: "TeamH", Winner: "TeamG"}, // QF pos 3
		{ID: "BracketId_SF1", Team1: "TeamA", Team2: "TeamC", Winner: "TeamA"}, // SF pos 4
		{ID: "BracketId_SF2", Team1: "TeamE", Team2: "TeamG", Winner: "TeamE"}, // SF pos 5
		{ID: "BracketId_GF1", Team1: "TeamA", Team2: "TeamE", Winner: "TeamA"}, // Final pos 6
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Len(t, results, 8)

	assert.Equal(t, "Grand Final", results["TeamA"].Round)
	assert.Equal(t, "advanced", results["TeamA"].Status)

	assert.Equal(t, "Grand Final", results["TeamE"].Round)
	assert.Equal(t, "eliminated", results["TeamE"].Status)

	assert.Equal(t, "Semi Final", results["TeamC"].Round)
	assert.Equal(t, "eliminated", results["TeamC"].Status)
	assert.Equal(t, "Semi Final", results["TeamG"].Round)
	assert.Equal(t, "eliminated", results["TeamG"].Status)

	assert.Equal(t, "Quarter Final", results["TeamB"].Round)
	assert.Equal(t, "eliminated", results["TeamB"].Status)
	assert.Equal(t, "Quarter Final", results["TeamD"].Round)
	assert.Equal(t, "eliminated", results["TeamD"].Status)
}

// TestGetEliminationResults_ModernID_Pending verifies pending status
// with modern opaque IDs when the Grand Final winner is not yet determined.
func TestGetEliminationResults_ModernID_Pending(t *testing.T) {
	matchNodes := []sources.MatchNode{
		{ID: "BracketId_GF1", Team1: "TeamA", Team2: "TeamB", Winner: "TBD"},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "Grand Final", results["TeamA"].Round)
	assert.Equal(t, "pending", results["TeamA"].Status)
}

// region roundFromIndex

func TestRoundFromIndex_Bracket8(t *testing.T) {
	// 7 matches, 3 rounds: QF(4) → SF(2) → Final(1)
	assert.Equal(t, 1, roundFromIndex(0, 7))
	assert.Equal(t, 1, roundFromIndex(3, 7))
	assert.Equal(t, 2, roundFromIndex(4, 7))
	assert.Equal(t, 2, roundFromIndex(5, 7))
	assert.Equal(t, 3, roundFromIndex(6, 7))
}

func TestRoundFromIndex_GrandFinalOnly(t *testing.T) {
	assert.Equal(t, 1, roundFromIndex(0, 1))
}

func TestRoundFromIndex_Bracket4(t *testing.T) {
	// 3 matches: SF(2) → Final(1)
	assert.Equal(t, 1, roundFromIndex(0, 3))
	assert.Equal(t, 1, roundFromIndex(1, 3))
	assert.Equal(t, 2, roundFromIndex(2, 3))
}

// endregion

// region bracketDepth

func TestBracketDepth(t *testing.T) {
	cases := []struct{ n, want int }{
		{1, 1},  // GF only
		{2, 1},  // GF + 1 extra (e.g. 3rd place with only SF+F)
		{3, 2},  // SF + GF
		{4, 2},  // SF + GF + 3rd place
		{7, 3},  // QF + SF + GF (Bracket/8, clean)
		{8, 3},  // Bracket/8 + 3rd-place match
		{15, 4}, // Bracket/16, clean
		{16, 4}, // Bracket/16 + 3rd-place
	}
	for _, c := range cases {
		assert.Equal(t, c.want, bracketDepth(c.n), "n=%d", c.n)
	}
}

// endregion

// region 3rd-place trimming

// TestGetEliminationResults_ThirdPlaceTrimmed verifies that a Bracket/8 with an
// extra 3rd-place consolation match (8 total) correctly assigns QF/SF/GF rounds.
func TestGetEliminationResults_ThirdPlaceTrimmed(t *testing.T) {
	matchNodes := []sources.MatchNode{
		// 4 QF (positions 0-3)
		{ID: "B_QF1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "B_QF2", Team1: "TeamC", Team2: "TeamD", Winner: "TeamC"},
		{ID: "B_QF3", Team1: "TeamE", Team2: "TeamF", Winner: "TeamE"},
		{ID: "B_QF4", Team1: "TeamG", Team2: "TeamH", Winner: "TeamG"},
		// 2 SF (positions 4-5)
		{ID: "B_SF1", Team1: "TeamA", Team2: "TeamC", Winner: "TeamA"},
		{ID: "B_SF2", Team1: "TeamE", Team2: "TeamG", Winner: "TeamE"},
		// Grand Final (position 6)
		{ID: "B_GF1", Team1: "TeamA", Team2: "TeamE", Winner: "TeamA"},
		// 3rd-place match — must be ignored for scoring
		{ID: "B_3P1", Team1: "TeamC", Team2: "TeamG", Winner: "TeamC"},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	// 8 teams expected (3rd place match teams still appear from earlier rounds)
	assert.Len(t, results, 8)

	// Champion should be at Grand Final, advanced
	assert.Equal(t, "Grand Final", results["TeamA"].Round)
	assert.Equal(t, "advanced", results["TeamA"].Status)

	// Runner-up at Grand Final, eliminated
	assert.Equal(t, "Grand Final", results["TeamE"].Round)
	assert.Equal(t, "eliminated", results["TeamE"].Status)

	// SF losers at Semi Final
	assert.Equal(t, "Semi Final", results["TeamC"].Round)
	assert.Equal(t, "Semi Final", results["TeamG"].Round)

	// QF losers at Quarter Final
	assert.Equal(t, "Quarter Final", results["TeamB"].Round)
	assert.Equal(t, "Quarter Final", results["TeamD"].Round)
	assert.Equal(t, "Quarter Final", results["TeamF"].Round)
	assert.Equal(t, "Quarter Final", results["TeamH"].Round)
}

// endregion

// TestGetEliminationResults_EmptyWinner tests with empty winner string
func TestGetEliminationResults_EmptyWinner(t *testing.T) {
	matchNodes := []sources.MatchNode{
		{ID: "bracket_R01-M001", Team1: "TeamA", Team2: "TeamB", Winner: ""},
	}
	results, err := getEliminationResults(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "pending", results["TeamA"].Status)
	assert.Equal(t, "pending", results["TeamB"].Status)
}

// TestGetEliminationResults_EmptyInput tests error with no matches
func TestGetEliminationResults_EmptyInput(t *testing.T) {
	_, err := getEliminationResults([]sources.MatchNode{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one match required")
}

// TestGetEliminationResults_EmptyTeamNames tests handling of empty team names
func TestGetEliminationResults_EmptyTeamNames(t *testing.T) {
	matchNodes := []sources.MatchNode{
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

// region NormalizeSingleElimSections

func TestNormalizeSingleElimSections_AllSame_Bracket8(t *testing.T) {
	// 7-node Bracket/8: all sections "Bracket/8" → should become QF/SF/GF
	nodes := []sources.MatchNode{
		{Section: "Bracket/8"}, // QF
		{Section: "Bracket/8"}, // QF
		{Section: "Bracket/8"}, // QF
		{Section: "Bracket/8"}, // QF
		{Section: "Bracket/8"}, // SF
		{Section: "Bracket/8"}, // SF
		{Section: "Bracket/8"}, // GF
	}
	got := NormalizeSingleElimSections(nodes)
	assert.Equal(t, "Quarterfinals", got[0].Section)
	assert.Equal(t, "Quarterfinals", got[3].Section)
	assert.Equal(t, "Semifinals", got[4].Section)
	assert.Equal(t, "Semifinals", got[5].Section)
	assert.Equal(t, "Grand Final", got[6].Section)
}

func TestNormalizeSingleElimSections_AlreadyVary_NoOp(t *testing.T) {
	// Sections already differ — function must not change them.
	nodes := []sources.MatchNode{
		{Section: "Quarterfinals"},
		{Section: "Quarterfinals"},
		{Section: "Semifinals"},
		{Section: "Grand Final"},
	}
	got := NormalizeSingleElimSections(nodes)
	assert.Equal(t, "Quarterfinals", got[0].Section)
	assert.Equal(t, "Semifinals", got[2].Section)
	assert.Equal(t, "Grand Final", got[3].Section)
}

func TestNormalizeSingleElimSections_Empty(t *testing.T) {
	got := NormalizeSingleElimSections(nil)
	assert.Empty(t, got)
}

// endregion

// region TrimSingleElimNodes

func TestTrimSingleElimNodes_NoOp(t *testing.T) {
	// 7 nodes (Bracket/8 with no consolation) — nothing to trim
	nodes := make([]sources.MatchNode, 7)
	got := TrimSingleElimNodes(nodes)
	assert.Len(t, got, 7)
}

func TestTrimSingleElimNodes_TrimsThirdPlace(t *testing.T) {
	// 8 nodes (Bracket/8 + 3rd-place consolation match)
	nodes := make([]sources.MatchNode, 8)
	got := TrimSingleElimNodes(nodes)
	assert.Len(t, got, 7)
}

func TestTrimSingleElimNodes_Empty(t *testing.T) {
	got := TrimSingleElimNodes(nil)
	assert.Empty(t, got)
}

// endregion
