//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"pickems-bot/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLeaderboard_Empty(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	seedGuild(t, "guild-1")
	tournamentID := seedTournament(t, "test-leaderboard-empty", "swiss")

	entries, err := s.GetLeaderboard(ctx, "guild-1", tournamentID)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestGetLeaderboard_WithScores(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-leaderboard-scores", "swiss")
	seedGuild(t, "guild-1")
	seedUser(t, "user-1", "Player1")
	seedUser(t, "user-2", "Player2")
	seedTeam(t, "TeamA", "test", "team-a")
	seedTeam(t, "TeamB", "test", "team-b")

	pred1 := models.Prediction{
		UserID:   "user-1",
		Username: "Player1",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamA"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-1", tournamentID, pred1))

	pred2 := models.Prediction{
		UserID:   "user-2",
		Username: "Player2",
		Round:    "Stage 1",
		Format:   "swiss",
		Win:      []string{"TeamB"},
	}
	require.NoError(t, s.UpsertPrediction(ctx, "guild-1", tournamentID, pred2))

	// Directly insert scores for both predictions
	_, err := testPool.Exec(ctx, `
		INSERT INTO scores (prediction_id, successes, pending, failed, last_computed_at)
		SELECT p.id, 3, 1, 0, $1
		FROM predictions p
		WHERE p.user_id = 'user-1' AND p.guild_id = 'guild-1' AND p.tournament_id = $2
	`, time.Now(), tournamentID)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `
		INSERT INTO scores (prediction_id, successes, pending, failed, last_computed_at)
		SELECT p.id, 1, 2, 1, $1
		FROM predictions p
		WHERE p.user_id = 'user-2' AND p.guild_id = 'guild-1' AND p.tournament_id = $2
	`, time.Now(), tournamentID)
	require.NoError(t, err)

	entries, err := s.GetLeaderboard(ctx, "guild-1", tournamentID)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// Ordered by successes DESC — user-1 (3) comes before user-2 (1)
	assert.Equal(t, "user-1", entries[0].UserID)
	assert.Equal(t, 3, entries[0].Successes)
	assert.Equal(t, 1, entries[0].Pending)
	assert.Equal(t, 0, entries[0].Failed)

	assert.Equal(t, "user-2", entries[1].UserID)
	assert.Equal(t, 1, entries[1].Successes)
	assert.Equal(t, 2, entries[1].Pending)
	assert.Equal(t, 1, entries[1].Failed)
}
