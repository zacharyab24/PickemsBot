/* swiss_test.go
 * Tests for the Swiss-format scoring path. Migrated from
 * api/logic/input_processing_test.go when the per-format scoring moved into
 * the format package.
 */

package tournament

import (
	"fmt"
	"testing"

	"pickems-bot/sources"
	"pickems-bot/models"

	"github.com/stretchr/testify/assert"
)

// region setSwissPredictions

// TestSetSwissPredictions checks that the input list is split into the correct
// bucket sizes for each supported Swiss bracket size.
// The input always has 5N/8 entries; buckets must be N/8, 3N/8, N/8.
func TestSetSwissPredictions(t *testing.T) {
	cases := []struct {
		totalTeams int
		wantWin    int
		wantAdv    int
		wantLose   int
	}{
		{8, 1, 3, 1},
		{16, 2, 6, 2},
		{24, 3, 9, 3},
		{32, 4, 12, 4},
	}
	for _, c := range cases {
		input := make([]string, 5*c.totalTeams/8)
		for i := range input {
			input[i] = fmt.Sprintf("team%d", i)
		}
		win, adv, lose := setSwissPredictions(input)
		assert.Len(t, win, c.wantWin, "totalTeams=%d win bucket", c.totalTeams)
		assert.Len(t, adv, c.wantAdv, "totalTeams=%d advance bucket", c.totalTeams)
		assert.Len(t, lose, c.wantLose, "totalTeams=%d lose bucket", c.totalTeams)
		// No teams should be silently dropped
		assert.Equal(t, len(input), len(win)+len(adv)+len(lose), "totalTeams=%d total coverage", c.totalTeams)
	}
}

// endregion

// region RequiredPredictions

// TestSwiss_RequiredPredictions table-tests the 5T/8 formula across realistic Swiss sizes.
func TestSwiss_RequiredPredictions(t *testing.T) {
	cases := []struct{ teams, want int }{
		{8, 5},
		{16, 10},
		{24, 15},
		{32, 20},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, swissFormat{}.RequiredPredictions(c.teams), "teamCount=%d", c.teams)
	}
}

// endregion

// region CalculateScore

// TestSwissCalculateScore_AllBucketsHappyPath exercises the 3-0, advance, and
// 0-3 buckets in one pass with a fully-correct prediction for a 16-team major.
func TestSwissCalculateScore_AllBucketsHappyPath(t *testing.T) {
	prediction := models.Prediction{
		Win:     []string{"A", "B"},
		Advance: []string{"C", "D", "E", "F", "G", "H"},
		Lose:    []string{"O", "P"},
	}
	results := SwissResult{Teams: map[string]string{
		"A": "3-0", "B": "3-0",
		"C": "3-1", "D": "3-1", "E": "3-2", "F": "3-2", "G": "3-1", "H": "3-2",
		"O": "0-3", "P": "0-3",
	}}

	report, err := swissFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	swissReport := report.(SwissReport)
	assert.Equal(t, 10, swissReport.Score.Successes)
	assert.Equal(t, 0, swissReport.Score.Pending)
	assert.Equal(t, 0, swissReport.Score.Failed)
	assert.Len(t, swissReport.WinPicks, 2)
	assert.Len(t, swissReport.AdvancePicks, 6)
	assert.Len(t, swissReport.LosePicks, 2)
}

// TestSwissCalculateScore_MissingTeam tests when a predicted team has no score in results.
func TestSwissCalculateScore_MissingTeam(t *testing.T) {
	prediction := models.Prediction{
		Win:     []string{"Team A"},
		Advance: []string{},
		Lose:    []string{},
	}
	results := SwissResult{Teams: map[string]string{}} // Team A missing

	report, err := swissFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	swissReport := report.(SwissReport)
	assert.Equal(t, 0, swissReport.Score.Successes)
	assert.Equal(t, 0, swissReport.Score.Pending)
	assert.Equal(t, 1, swissReport.Score.Failed)
	assert.Equal(t, "", swissReport.WinPicks[0].Score) // empty score signals missing
	assert.Equal(t, StatusFailed, swissReport.WinPicks[0].Status)
}

// TestSwissCalculateScore_InvalidScoreFormat tests handling of malformed score strings.
func TestSwissCalculateScore_InvalidScoreFormat(t *testing.T) {
	prediction := models.Prediction{Win: []string{"Team A"}}
	results := SwissResult{Teams: map[string]string{"Team A": "invalid"}}

	_, err := swissFormat{}.CalculateScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid score format")
}

// TestSwissCalculateScore_InvalidScoreNumbers tests handling of non-numeric scores.
func TestSwissCalculateScore_InvalidScoreNumbers(t *testing.T) {
	prediction := models.Prediction{Win: []string{"Team A"}}
	results := SwissResult{Teams: map[string]string{"Team A": "a-b"}}

	_, err := swissFormat{}.CalculateScore(prediction, results)

	assert.Error(t, err)
}

// TestSwissCalculateScore_AdvanceCategory3_0 tests that 3-0 teams fail the advance bucket.
func TestSwissCalculateScore_AdvanceCategory3_0(t *testing.T) {
	prediction := models.Prediction{Advance: []string{"Team A"}}
	results := SwissResult{Teams: map[string]string{"Team A": "3-0"}}

	report, err := swissFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	swissReport := report.(SwissReport)
	assert.Equal(t, 0, swissReport.Score.Successes)
	assert.Equal(t, 1, swissReport.Score.Failed)
}

// TestSwissCalculateScore_AdvanceCategory0_3 tests that 0-3 teams fail the advance bucket.
func TestSwissCalculateScore_AdvanceCategory0_3(t *testing.T) {
	prediction := models.Prediction{Advance: []string{"Team A"}}
	results := SwissResult{Teams: map[string]string{"Team A": "0-3"}}

	report, err := swissFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	swissReport := report.(SwissReport)
	assert.Equal(t, 0, swissReport.Score.Successes)
	assert.Equal(t, 1, swissReport.Score.Failed)
}

// TestSwissCalculateScore_WrongResultType ensures we error rather than panic
// when a non-SwissResult is dispatched here.
func TestSwissCalculateScore_WrongResultType(t *testing.T) {
	_, err := swissFormat{}.CalculateScore(models.Prediction{}, EliminationResult{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected SwissResult")
}

// endregion

// region evaluateBucket

// TestEvaluateBucket_AllScenarios exercises the helper across the three statuses.
func TestEvaluateBucket_AllScenarios(t *testing.T) {
	teams := []string{"Team A", "Team B", "Team C"}
	scores := map[string]string{
		"Team A": "3-0",
		"Team B": "2-0",
		"Team C": "1-2",
	}

	classify := func(wins, loses int) BucketStatus {
		if loses >= 1 {
			return StatusFailed
		} else if wins != 3 {
			return StatusPending
		}
		return StatusSucceeded
	}

	entries, err := evaluateBucket(teams, scores, classify)

	assert.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, StatusSucceeded, entries[0].Status) // Team A: 3-0
	assert.Equal(t, StatusPending, entries[1].Status)   // Team B: 2-0
	assert.Equal(t, StatusFailed, entries[2].Status)    // Team C: 1-2
}

// endregion

// FromRecord/ToRecord tests removed: those methods no longer exist after the
// in-memory result and storage record types were unified.

// region calculateSwissScores

// TestCalculateSwissScores_AllWins tests team with all wins
func TestCalculateSwissScores_AllWins(t *testing.T) {
	matchNodes := []sources.MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "2", Team1: "TeamA", Team2: "TeamC", Winner: "TeamA"},
		{ID: "3", Team1: "TeamA", Team2: "TeamD", Winner: "TeamA"},
	}
	scores, err := calculateSwissScores(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "3-0", scores["TeamA"])
	assert.Equal(t, "0-1", scores["TeamB"])
	assert.Equal(t, "0-1", scores["TeamC"])
	assert.Equal(t, "0-1", scores["TeamD"])
}

// TestCalculateSwissScores_MixedResults tests mixed win/loss records
func TestCalculateSwissScores_MixedResults(t *testing.T) {
	matchNodes := []sources.MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "2", Team1: "TeamA", Team2: "TeamC", Winner: "TeamC"},
		{ID: "3", Team1: "TeamB", Team2: "TeamC", Winner: "TeamB"},
	}
	scores, err := calculateSwissScores(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "1-1", scores["TeamA"])
	assert.Equal(t, "1-1", scores["TeamB"])
	assert.Equal(t, "1-1", scores["TeamC"])
}

// TestCalculateSwissScores_PendingMatches tests with TBD winners (pending matches)
func TestCalculateSwissScores_PendingMatches(t *testing.T) {
	matchNodes := []sources.MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
		{ID: "2", Team1: "TeamA", Team2: "TeamC", Winner: "TBD"},
		{ID: "3", Team1: "TeamB", Team2: "TeamC", Winner: "TBD"},
	}
	scores, err := calculateSwissScores(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "1-0", scores["TeamA"])
	assert.Equal(t, "0-1", scores["TeamB"])
	assert.Equal(t, "0-0", scores["TeamC"])
}

// TestCalculateSwissScores_EmptyInput tests with no matches
func TestCalculateSwissScores_EmptyInput(t *testing.T) {
	scores, err := calculateSwissScores([]sources.MatchNode{})
	assert.NoError(t, err)
	assert.Empty(t, scores)
}

// TestCalculateSwissScores_TBDTeamsExcluded tests that TBD teams are excluded
func TestCalculateSwissScores_TBDTeamsExcluded(t *testing.T) {
	matchNodes := []sources.MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TBD", Winner: "TeamA"},
		{ID: "2", Team1: "TBD", Team2: "TeamB", Winner: "TeamB"},
	}
	scores, err := calculateSwissScores(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "1-0", scores["TeamA"])
	assert.Equal(t, "1-0", scores["TeamB"])
	_, hasTBD := scores["TBD"]
	assert.False(t, hasTBD)
}

// TestCalculateSwissScores_InvalidWinner tests handling of unexpected winner values
func TestCalculateSwissScores_InvalidWinner(t *testing.T) {
	matchNodes := []sources.MatchNode{
		{ID: "1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamC"}, // Invalid winner
		{ID: "2", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA"},
	}
	scores, err := calculateSwissScores(matchNodes)
	assert.NoError(t, err)
	assert.Equal(t, "1-0", scores["TeamA"])
	assert.Equal(t, "0-1", scores["TeamB"])
}

// endregion
