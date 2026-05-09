/* swiss_test.go
 * Tests for the Swiss-format scoring path. Migrated from
 * api/logic/input_processing_test.go when the per-format scoring moved into
 * the format package.
 */

package format

import (
	"strings"
	"testing"

	"pickems-bot/api/shared"

	"github.com/stretchr/testify/assert"
)

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
	prediction := shared.Prediction{
		Win:     []string{"A", "B"},
		Advance: []string{"C", "D", "E", "F", "G", "H"},
		Lose:    []string{"O", "P"},
	}
	results := SwissResult{Teams: map[string]string{
		"A": "3-0", "B": "3-0",
		"C": "3-1", "D": "3-1", "E": "3-2", "F": "3-2", "G": "3-1", "H": "3-2",
		"O": "0-3", "P": "0-3",
	}}

	scoreResult, report, err := swissFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 10, scoreResult.Successes)
	assert.Equal(t, 0, scoreResult.Pending)
	assert.Equal(t, 0, scoreResult.Failed)
	assert.Contains(t, report, "[3-0]")
	assert.Contains(t, report, "[3-1, 3-2]")
	assert.Contains(t, report, "[0-3]")
}

// TestSwissCalculateScore_MissingTeam tests when a predicted team has no score in results.
func TestSwissCalculateScore_MissingTeam(t *testing.T) {
	prediction := shared.Prediction{
		Win:     []string{"Team A"},
		Advance: []string{},
		Lose:    []string{},
	}
	results := SwissResult{Teams: map[string]string{}} // Team A missing

	scoreResult, report, err := swissFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 0, scoreResult.Pending)
	assert.Equal(t, 1, scoreResult.Failed)
	assert.Contains(t, report, "Missing score")
}

// TestSwissCalculateScore_InvalidScoreFormat tests handling of malformed score strings.
func TestSwissCalculateScore_InvalidScoreFormat(t *testing.T) {
	prediction := shared.Prediction{Win: []string{"Team A"}}
	results := SwissResult{Teams: map[string]string{"Team A": "invalid"}}

	_, _, err := swissFormat{}.CalculateScore(prediction, results)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid score format")
}

// TestSwissCalculateScore_InvalidScoreNumbers tests handling of non-numeric scores.
func TestSwissCalculateScore_InvalidScoreNumbers(t *testing.T) {
	prediction := shared.Prediction{Win: []string{"Team A"}}
	results := SwissResult{Teams: map[string]string{"Team A": "a-b"}}

	_, _, err := swissFormat{}.CalculateScore(prediction, results)

	assert.Error(t, err)
}

// TestSwissCalculateScore_AdvanceCategory3_0 tests that 3-0 teams fail the advance bucket.
func TestSwissCalculateScore_AdvanceCategory3_0(t *testing.T) {
	prediction := shared.Prediction{Advance: []string{"Team A"}}
	results := SwissResult{Teams: map[string]string{"Team A": "3-0"}}

	scoreResult, _, err := swissFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 1, scoreResult.Failed)
}

// TestSwissCalculateScore_AdvanceCategory0_3 tests that 0-3 teams fail the advance bucket.
func TestSwissCalculateScore_AdvanceCategory0_3(t *testing.T) {
	prediction := shared.Prediction{Advance: []string{"Team A"}}
	results := SwissResult{Teams: map[string]string{"Team A": "0-3"}}

	scoreResult, _, err := swissFormat{}.CalculateScore(prediction, results)

	assert.NoError(t, err)
	assert.Equal(t, 0, scoreResult.Successes)
	assert.Equal(t, 1, scoreResult.Failed)
}

// TestSwissCalculateScore_WrongResultType ensures we error rather than panic
// when a non-SwissResult is dispatched here.
func TestSwissCalculateScore_WrongResultType(t *testing.T) {
	_, _, err := swissFormat{}.CalculateScore(shared.Prediction{}, EliminationResult{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected SwissResult")
}

// endregion

// region evaluateBucket

// TestEvaluateBucket_AllScenarios exercises the helper across the three statuses.
func TestEvaluateBucket_AllScenarios(t *testing.T) {
	var builder strings.Builder
	teams := []string{"Team A", "Team B", "Team C"}
	scores := map[string]string{
		"Team A": "3-0",
		"Team B": "2-0",
		"Team C": "1-2",
	}

	classify := func(wins, loses int) bucketStatus {
		if loses >= 1 {
			return statusFailed
		} else if wins != 3 {
			return statusPending
		}
		return statusSucceeded
	}

	scoreResult, err := evaluateBucket(teams, scores, classify, &builder)

	assert.NoError(t, err)
	assert.Equal(t, 1, scoreResult.Successes) // Team A
	assert.Equal(t, 1, scoreResult.Pending)   // Team B
	assert.Equal(t, 1, scoreResult.Failed)    // Team C
}

// endregion

// FromRecord/ToRecord tests removed: those methods no longer exist after the
// in-memory result and storage record types were unified.
