/* pandascore_test.go
 * Contains unit tests for pandascore.go functions
 * Authors: Zachary Bower
 */

package sources

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// region ErrUnrecoverable tests

func TestErrUnrecoverable_Sentinel(t *testing.T) {
	// Wrapping ErrUnrecoverable should still be detectable via errors.Is
	wrapped := fmt.Errorf("%w: status 401", ErrUnrecoverable)
	assert.True(t, errors.Is(wrapped, ErrUnrecoverable))
}

func TestErrUnrecoverable_UnrelatedError(t *testing.T) {
	// A plain error should not match ErrUnrecoverable
	plain := fmt.Errorf("some other error")
	assert.False(t, errors.Is(plain, ErrUnrecoverable))
}

// endregion

// region parsePandaScoreMatch tests

func TestParsePandaScoreMatch_FinishedStatus(t *testing.T) {
	match := map[string]interface{}{
		"id":     float64(12345),
		"name":   "Round 1",
		"status": "finished",
		"opponents": []interface{}{
			map[string]interface{}{
				"opponent": map[string]interface{}{"name": "Team A", "id": float64(1)},
			},
			map[string]interface{}{
				"opponent": map[string]interface{}{"name": "Team B", "id": float64(2)},
			},
		},
		"winner": map[string]interface{}{"name": "Team A"},
		"results": []interface{}{
			map[string]interface{}{"team_id": float64(1), "score": float64(2)},
			map[string]interface{}{"team_id": float64(2), "score": float64(0)},
		},
	}

	node, err := parsePandaScoreMatch(match)

	require.NoError(t, err)
	assert.Equal(t, "finished", node.Status)
	assert.Equal(t, "Team A", node.Winner)
	assert.Equal(t, "2-0", node.Score)
}

func TestParsePandaScoreMatch_NotStartedStatus(t *testing.T) {
	match := map[string]interface{}{
		"id":     float64(12346),
		"name":   "Round 1",
		"status": "not_started",
		"opponents": []interface{}{
			map[string]interface{}{
				"opponent": map[string]interface{}{"name": "Team A", "id": float64(1)},
			},
			map[string]interface{}{
				"opponent": map[string]interface{}{"name": "Team B", "id": float64(2)},
			},
		},
		"winner":  nil,
		"results": []interface{}{},
	}

	node, err := parsePandaScoreMatch(match)

	require.NoError(t, err)
	assert.Equal(t, "not_started", node.Status)
	assert.Equal(t, "TBD", node.Winner)
	assert.Equal(t, "", node.Score)
}

func TestParsePandaScoreMatch_RunningStatus(t *testing.T) {
	match := map[string]interface{}{
		"id":     float64(12347),
		"name":   "Round 2",
		"status": "running",
		"opponents": []interface{}{
			map[string]interface{}{
				"opponent": map[string]interface{}{"name": "NaVi", "id": float64(3)},
			},
			map[string]interface{}{
				"opponent": map[string]interface{}{"name": "FaZe", "id": float64(4)},
			},
		},
		"winner":  nil,
		"results": []interface{}{},
	}

	node, err := parsePandaScoreMatch(match)

	require.NoError(t, err)
	assert.Equal(t, "running", node.Status)
}

func TestParsePandaScoreMatch_TBDOpponents(t *testing.T) {
	// Matches with no opponents yet should default to "TBD"
	match := map[string]interface{}{
		"id":        float64(99999),
		"name":      "Grand Final",
		"status":    "not_started",
		"opponents": []interface{}{},
		"winner":    nil,
		"results":   []interface{}{},
	}

	node, err := parsePandaScoreMatch(match)

	require.NoError(t, err)
	assert.Equal(t, "TBD", node.Team1)
	assert.Equal(t, "TBD", node.Team2)
}

// endregion

// region ParsePandaScoreMatches tests

func TestParsePandaScoreMatches_StatusPopulated(t *testing.T) {
	// Verify Status is populated on all nodes when parsing a slice
	json := `[
		{"id": 1, "name": "Match 1", "status": "finished",
		 "opponents": [
			{"opponent": {"name": "Alpha", "id": 1}},
			{"opponent": {"name": "Beta",  "id": 2}}
		 ],
		 "winner": {"name": "Alpha"},
		 "results": [{"team_id": 1, "score": 2}, {"team_id": 2, "score": 1}]},
		{"id": 2, "name": "Match 2", "status": "not_started",
		 "opponents": [
			{"opponent": {"name": "Gamma", "id": 3}},
			{"opponent": {"name": "Delta", "id": 4}}
		 ],
		 "winner": null, "results": []}
	]`

	nodes, err := ParsePandaScoreMatches(json)

	require.NoError(t, err)
	require.Len(t, nodes, 2)
	assert.Equal(t, "finished", nodes[0].Status)
	assert.Equal(t, "not_started", nodes[1].Status)
}

// endregion

// region ParsePandaScoreSchedule tests

func TestParsePandaScoreSchedule_WithStream(t *testing.T) {
	json := `[{
		"status": "not_started",
		"scheduled_at": "2025-11-15T14:00:00Z",
		"number_of_games": 3,
		"opponents": [
			{"opponent": {"name": "Team A"}},
			{"opponent": {"name": "Team B"}}
		],
		"streams_list": [
			{"main": true, "official": true, "raw_url": "https://www.twitch.tv/blastpremier"},
			{"main": false, "official": true, "raw_url": "https://www.youtube.com/blast"}
		]
	}]`

	matches, err := ParsePandaScoreSchedule(json)

	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "https://www.twitch.tv/blastpremier", matches[0].StreamURL)
}

func TestParsePandaScoreSchedule_NoStream(t *testing.T) {
	// Matches with no streams assigned yet should have empty StreamURL
	json := `[{
		"status": "not_started",
		"scheduled_at": "2025-11-15T14:00:00Z",
		"number_of_games": 3,
		"opponents": [
			{"opponent": {"name": "Team A"}},
			{"opponent": {"name": "Team B"}}
		],
		"streams_list": []
	}]`

	matches, err := ParsePandaScoreSchedule(json)

	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "", matches[0].StreamURL)
}

func TestParsePandaScoreSchedule_FinishedMatch(t *testing.T) {
	json := `[{
		"status": "finished",
		"scheduled_at": "2025-11-10T12:00:00Z",
		"number_of_games": 1,
		"opponents": [
			{"opponent": {"name": "Team X"}},
			{"opponent": {"name": "Team Y"}}
		],
		"streams_list": []
	}]`

	matches, err := ParsePandaScoreSchedule(json)

	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.True(t, matches[0].Finished)
	assert.Equal(t, "1", matches[0].BestOf)
}

// endregion
