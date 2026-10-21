package tournament

import (
	"fmt"
	"testing"

	"pickems-bot/models"
	"pickems-bot/sources"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwissFormat_GeneratePrediction_SplitsIntoBuckets(t *testing.T) {
	teams := make([]string, 10)
	for i := range teams {
		teams[i] = fmt.Sprintf("team%d", i)
	}

	p, err := swissFormat{}.GeneratePrediction(models.User{UserID: "u1", Username: "alice"}, "Stage 1", teams)
	require.NoError(t, err)

	assert.Equal(t, "u1", p.UserID)
	assert.Equal(t, "alice", p.Username)
	assert.Equal(t, string(Swiss), p.Format)
	assert.Equal(t, "Stage 1", p.Round)
	assert.Equal(t, []string{"team0", "team1"}, p.Win)
	assert.Equal(t, []string{"team2", "team3", "team4", "team5", "team6", "team7"}, p.Advance)
	assert.Equal(t, []string{"team8", "team9"}, p.Lose)
}

// Teams arrive ordered 3rd/4th, 3rd/4th, runner-up, winner - the order the
// /set submit handler concatenates its select menus in.
func TestSingleElimFormat_GeneratePrediction_MapsPicksToFinishingRound(t *testing.T) {
	p, err := singleElimFormat{}.GeneratePrediction(models.User{UserID: "u2"}, "Playoffs", []string{"T1", "T2", "T3", "T4"})
	require.NoError(t, err)

	assert.Equal(t, string(SingleElim), p.Format)
	assert.Equal(t, map[string]models.TeamProgress{
		"T4": {Round: "Grand Final", Status: "advanced"},
		"T3": {Round: "Grand Final", Status: "eliminated"},
		"T2": {Round: "Semi Final", Status: "eliminated"},
		"T1": {Round: "Semi Final", Status: "eliminated"},
	}, p.Progression)
}

func TestSwissFormat_BuildFromMatchNodes_TalliesRecords(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Section: "Round 1"},
		{Team1: "C", Team2: "D", Winner: "C", Section: "Round 1"},
		{Team1: "A", Team2: "C", Winner: "C", Section: "Round 2"},
	}

	result, err := swissFormat{}.BuildFromMatchNodes(nodes, "Stage 1")
	require.NoError(t, err)

	sr, ok := result.(SwissResult)
	require.True(t, ok)
	assert.Equal(t, "Stage 1", sr.Round)
	assert.Equal(t, map[string]string{"A": "1-1", "B": "0-1", "C": "2-0", "D": "0-1"}, sr.Teams)
}

func TestSingleElimFormat_BuildFromMatchNodes_TracksProgression(t *testing.T) {
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
	assert.Equal(t, models.TeamProgress{Round: "Grand Final", Status: "advanced"}, er.Teams["A"])
	assert.Equal(t, models.TeamProgress{Round: "Grand Final", Status: "eliminated"}, er.Teams["C"])
	assert.Equal(t, "eliminated", er.Teams["B"].Status)
	assert.Equal(t, "eliminated", er.Teams["D"].Status)
}

func TestSingleElimFormat_BuildFromMatchNodes_EmptyErrors(t *testing.T) {
	_, err := singleElimFormat{}.BuildFromMatchNodes(nil, "Playoffs")
	assert.Error(t, err)
}
