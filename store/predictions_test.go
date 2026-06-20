//go:build integration

package store

import (
	"context"
	"testing"

	"pickems-bot/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertPrediction_Swiss(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-swiss", "swiss")
	seedGuild(t, "guild-1")
	seedUser(t, "user-1", "Player1")
	seedTeam(t, "TeamA", "test", "team-a")
	seedTeam(t, "TeamB", "test", "team-b")
	seedTeam(t, "TeamC", "test", "team-c")

	pred := models.Prediction{
		UserID:   "user-1",
		Username: "Player1",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamA"},
		Advance:  []string{"TeamB"},
		Lose:     []string{"TeamC"},
	}

	err := s.UpsertPrediction(ctx, "guild-1", tournamentID, pred)
	require.NoError(t, err)

	got, err := s.GetPrediction(ctx, "user-1", "guild-1", tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Equal(t, "Stage 1", got.Round)
	assert.Equal(t, "swiss", got.Format)
	assert.Contains(t, got.Win, "TeamA")
	assert.Contains(t, got.Advance, "TeamB")
	assert.Contains(t, got.Lose, "TeamC")
}

func TestUpsertPrediction_Idempotent(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-idempotent", "swiss")
	seedGuild(t, "guild-1")
	seedUser(t, "user-1", "Player1")
	seedTeam(t, "TeamA", "test", "team-a")
	seedTeam(t, "TeamB", "test", "team-b")
	seedTeam(t, "TeamC", "test", "team-c")
	seedTeam(t, "TeamD", "test", "team-d")
	seedTeam(t, "TeamE", "test", "team-e")
	seedTeam(t, "TeamF", "test", "team-f")

	first := models.Prediction{
		UserID:   "user-1",
		Username: "Player1",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamA"},
		Advance:  []string{"TeamB"},
		Lose:     []string{"TeamC"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-1", tournamentID, first))

	second := models.Prediction{
		UserID:   "user-1",
		Username: "Player1",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamD"},
		Advance:  []string{"TeamE"},
		Lose:     []string{"TeamF"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-1", tournamentID, second))

	got, err := s.GetPrediction(ctx, "user-1", "guild-1", tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Contains(t, got.Win, "TeamD")
	assert.Contains(t, got.Advance, "TeamE")
	assert.Contains(t, got.Lose, "TeamF")
	assert.NotContains(t, got.Win, "TeamA")
}

func TestGetPrediction_NotFound(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-notfound", "swiss")

	_, err := s.GetPrediction(ctx, "nonexistent-user", "guild-x", tournamentID, "Stage 1")
	assert.Error(t, err)
}

func TestGetPredictionByUsername_CaseInsensitive(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-caseinsensitive", "swiss")
	seedGuild(t, "guild-1")
	seedUser(t, "user-1", "TestUser")
	seedTeam(t, "TeamA", "test", "team-a")

	pred := models.Prediction{
		UserID:   "user-1",
		Username: "TestUser",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamA"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-1", tournamentID, pred))

	got, err := s.GetPredictionByUsername(ctx, "testuser", "guild-1", tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Equal(t, "user-1", got.UserID)
	assert.Contains(t, got.Win, "TeamA")
}

func TestListPredictions_ByGuild(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-listbyguild", "swiss")
	seedGuild(t, "guild-A")
	seedGuild(t, "guild-B")
	seedUser(t, "user-1", "Player1")
	seedUser(t, "user-2", "Player2")
	seedTeam(t, "TeamA", "test", "team-a")
	seedTeam(t, "TeamB", "test", "team-b")

	predA := models.Prediction{
		UserID:   "user-1",
		Username: "Player1",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamA"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-A", tournamentID, predA))

	predB := models.Prediction{
		UserID:   "user-2",
		Username: "Player2",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamB"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-B", tournamentID, predB))

	results, err := s.ListPredictions(ctx, "guild-A", tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "user-1", results[0].UserID)
}

func TestListPredictions_AllGuilds(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-listallguilds", "swiss")
	seedGuild(t, "guild-A")
	seedGuild(t, "guild-B")
	seedUser(t, "user-1", "Player1")
	seedUser(t, "user-2", "Player2")
	seedTeam(t, "TeamA", "test", "team-a")
	seedTeam(t, "TeamB", "test", "team-b")

	predA := models.Prediction{
		UserID:   "user-1",
		Username: "Player1",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamA"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-A", tournamentID, predA))

	predB := models.Prediction{
		UserID:   "user-2",
		Username: "Player2",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamB"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-B", tournamentID, predB))

	results, err := s.ListPredictions(ctx, "", tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestListValidTeams(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-validteams", "swiss")
	seedMatch(t, tournamentID, "Stage 1", "TeamAlpha", "TeamBeta", "pending")
	seedMatch(t, tournamentID, "Stage 1", "TeamGamma", "TeamDelta", "pending")

	teams, kind, err := s.ListValidTeams(ctx, tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Equal(t, "swiss", string(kind))
	assert.Len(t, teams, 4)
	assert.Contains(t, teams, "TeamAlpha")
	assert.Contains(t, teams, "TeamBeta")
	assert.Contains(t, teams, "TeamGamma")
	assert.Contains(t, teams, "TeamDelta")
}
