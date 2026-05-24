/* getters_test.go
 * Tests for getter methods and simple functions across Swiss and SingleElim formats.
 */

package tournament

import (
	"fmt"
	"testing"

	"pickems-bot/models"
	"pickems-bot/sources"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

// region BucketStatus.String

func TestBucketStatus_String_AllValues(t *testing.T) {
	cases := []struct {
		status BucketStatus
		want   string
	}{
		{StatusSucceeded, "✅"},
		{StatusPending, "⏳"},
		{StatusFailed, "❌"},
		{BucketStatus(99), "❓"}, // unknown value → fallback
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.status.String(), "BucketStatus(%d).String()", c.status)
	}
}

// endregion

// region SwissReport methods

func TestSwissReport_FormatKind(t *testing.T) {
	r := SwissReport{}
	assert.Equal(t, Swiss, r.FormatKind())
}

func TestSwissReport_GetScore(t *testing.T) {
	r := SwissReport{Score: models.ScoreResult{Successes: 3, Pending: 1, Failed: 2}}
	s := r.GetScore()
	assert.Equal(t, 3, s.Successes)
	assert.Equal(t, 1, s.Pending)
	assert.Equal(t, 2, s.Failed)
}

// endregion

// region SwissResult methods

func TestSwissResult_GetType(t *testing.T) {
	r := SwissResult{}
	assert.Equal(t, Swiss, r.GetType())
}

func TestSwissResult_GetRound(t *testing.T) {
	r := SwissResult{Round: "Stage 1"}
	assert.Equal(t, "Stage 1", r.GetRound())
}

func TestSwissResult_GetTeamNames(t *testing.T) {
	r := SwissResult{Teams: map[string]string{"Alpha": "3-0", "Beta": "2-1"}}
	names := r.GetTeamNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "Alpha")
	assert.Contains(t, names, "Beta")
}

// endregion

// region swissFormat.GeneratePrediction / BuildFromMatchNodes / DecodeBSON

func TestSwissFormat_GeneratePrediction(t *testing.T) {
	user := models.User{UserID: "u1", Username: "alice"}
	// 10 teams for a 16-team Swiss (5×16/8 = 10)
	teams := make([]string, 10)
	for i := range teams {
		teams[i] = fmt.Sprintf("team%d", i)
	}
	p, err := swissFormat{}.GeneratePrediction(user, "Round 1", teams)
	require.NoError(t, err)
	assert.Equal(t, "u1", p.UserID)
	assert.Equal(t, "alice", p.Username)
	assert.Equal(t, "swiss", p.Format)
	assert.Equal(t, "Round 1", p.Round)
	assert.Len(t, p.Win, 2)     // 10/5 = 2
	assert.Len(t, p.Advance, 6) // 3×2 = 6
	assert.Len(t, p.Lose, 2)    // 10/5 = 2
}

func TestSwissFormat_BuildFromMatchNodes_Success(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Section: "Round 1"},
		{Team1: "C", Team2: "D", Winner: "C", Section: "Round 1"},
	}
	result, err := swissFormat{}.BuildFromMatchNodes(nodes, "Stage 1")
	require.NoError(t, err)
	sr, ok := result.(SwissResult)
	require.True(t, ok)
	assert.Equal(t, "Stage 1", sr.Round)
	assert.Equal(t, "1-0", sr.Teams["A"])
	assert.Equal(t, "0-1", sr.Teams["B"])
}

func TestSwissFormat_DecodeBSON_RoundTrip(t *testing.T) {
	original := SwissResult{
		Round: "Stage 2",
		Teams: map[string]string{"Alpha": "2-1", "Beta": "1-2"},
	}
	raw, err := bson.Marshal(original)
	require.NoError(t, err)

	decoded, err := swissFormat{}.DecodeBSON(raw)
	require.NoError(t, err)
	sr, ok := decoded.(SwissResult)
	require.True(t, ok)
	assert.Equal(t, "Stage 2", sr.Round)
	assert.Equal(t, "2-1", sr.Teams["Alpha"])
}

func TestSwissFormat_DecodeBSON_InvalidBytes(t *testing.T) {
	_, err := swissFormat{}.DecodeBSON([]byte("not valid bson data!!"))
	assert.Error(t, err)
}

// endregion

// region SingleElimReport methods

func TestSingleElimReport_FormatKind(t *testing.T) {
	r := SingleElimReport{}
	assert.Equal(t, SingleElim, r.FormatKind())
}

func TestSingleElimReport_GetScore(t *testing.T) {
	r := SingleElimReport{Score: models.ScoreResult{Successes: 2, Pending: 0, Failed: 1}}
	s := r.GetScore()
	assert.Equal(t, 2, s.Successes)
	assert.Equal(t, 0, s.Pending)
	assert.Equal(t, 1, s.Failed)
}

// endregion

// region EliminationResult methods

func TestEliminationResult_GetType(t *testing.T) {
	r := EliminationResult{}
	assert.Equal(t, SingleElim, r.GetType())
}

func TestEliminationResult_GetRound(t *testing.T) {
	r := EliminationResult{Round: "Playoffs"}
	assert.Equal(t, "Playoffs", r.GetRound())
}

func TestEliminationResult_GetTeamNames(t *testing.T) {
	r := EliminationResult{Teams: map[string]models.TeamProgress{
		"Alpha": {Round: "Grand Final", Status: "advanced"},
		"Beta":  {Round: "Semi Final", Status: "eliminated"},
	}}
	names := r.GetTeamNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "Alpha")
	assert.Contains(t, names, "Beta")
}

// endregion

// region singleElimFormat.GeneratePrediction / BuildFromMatchNodes / DecodeBSON

func TestSingleElimFormat_GeneratePrediction(t *testing.T) {
	user := models.User{UserID: "u2", Username: "bob"}
	// 4 teams → last-to-first: predicted champion + semi runners + quarter etc.
	teams := []string{"T1", "T2", "T3", "T4"}
	p, err := singleElimFormat{}.GeneratePrediction(user, "Playoffs", teams)
	require.NoError(t, err)
	assert.Equal(t, "u2", p.UserID)
	assert.Equal(t, "single-elimination", p.Format)
	assert.Equal(t, "Playoffs", p.Round)
	assert.NotEmpty(t, p.Progression)
	// 4 teams → 4 entries in progression map
	assert.Len(t, p.Progression, 4)
}

func TestSingleElimFormat_BuildFromMatchNodes_Success(t *testing.T) {
	// 3-match bracket: 2 semi-finals then 1 grand final
	nodes := []sources.MatchNode{
		{ID: "BR_R01-M001", Team1: "A", Team2: "B", Winner: "A", Section: "Bracket/4"},
		{ID: "BR_R01-M002", Team1: "C", Team2: "D", Winner: "C", Section: "Bracket/4"},
		{ID: "BR_R02-M001", Team1: "A", Team2: "C", Winner: "A", Section: "Bracket/4"},
	}
	result, err := singleElimFormat{}.BuildFromMatchNodes(nodes, "Playoffs")
	require.NoError(t, err)
	er, ok := result.(EliminationResult)
	require.True(t, ok)
	assert.Equal(t, "Playoffs", er.Round)
	assert.Equal(t, "advanced", er.Teams["A"].Status)
	assert.Equal(t, "eliminated", er.Teams["C"].Status)
}

func TestSingleElimFormat_BuildFromMatchNodes_Empty(t *testing.T) {
	_, err := singleElimFormat{}.BuildFromMatchNodes(nil, "Playoffs")
	assert.Error(t, err)
}

func TestSingleElimFormat_DecodeBSON_RoundTrip(t *testing.T) {
	original := EliminationResult{
		Round: "Playoffs",
		Teams: map[string]models.TeamProgress{
			"Alpha": {Round: "Grand Final", Status: "advanced"},
		},
	}
	raw, err := bson.Marshal(original)
	require.NoError(t, err)

	decoded, err := singleElimFormat{}.DecodeBSON(raw)
	require.NoError(t, err)
	er, ok := decoded.(EliminationResult)
	require.True(t, ok)
	assert.Equal(t, "Playoffs", er.Round)
	assert.Equal(t, "advanced", er.Teams["Alpha"].Status)
}

func TestSingleElimFormat_DecodeBSON_InvalidBytes(t *testing.T) {
	_, err := singleElimFormat{}.DecodeBSON([]byte("not valid bson!!"))
	assert.Error(t, err)
}

// endregion
