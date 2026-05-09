/* single_elimination_test.go
 * Tests for the single-elimination format scoring path.
 */

package format

import (
	"testing"

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
