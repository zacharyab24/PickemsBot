//go:build integration

package store

import (
	"context"
	"testing"

	"pickems-bot/sources"
	"pickems-bot/tournament"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMatchNodes_ReturnsNodes(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-nodes", "swiss")
	seedMatch(t, tournamentID, "Stage 1", "TeamA", "TeamB", "pending")
	seedMatch(t, tournamentID, "Stage 1", "TeamC", "TeamD", "pending")

	nodes, kind, err := s.GetMatchNodes(ctx, tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Equal(t, "swiss", string(kind))
	assert.Len(t, nodes, 2)

	teams := make(map[string]bool)
	for _, n := range nodes {
		teams[n.Team1] = true
		teams[n.Team2] = true
	}
	assert.True(t, teams["TeamA"])
	assert.True(t, teams["TeamB"])
	assert.True(t, teams["TeamC"])
	assert.True(t, teams["TeamD"])
}

func TestGetMatchNodes_EmptyRound(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-empty-round", "swiss")

	_, _, err := s.GetMatchNodes(ctx, tournamentID, "Stage 1")
	assert.Error(t, err)
}

func TestGetMatchNodes_WinnerResolvedFromFK(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-winner-fk", "swiss")
	winnerTeamID := seedTeam(t, "TeamA", "test", "team-a")

	// Seed a completed match with score and winner FK set directly.
	var matchID int
	err := testPool.QueryRow(ctx, `
		INSERT INTO matches (tournament_id, round, team1_name, team2_name, score, status, winner_id)
		VALUES ($1, 'Stage 1', 'TeamA', 'TeamB', '3-1', 'completed', $2)
		RETURNING id
	`, tournamentID, winnerTeamID).Scan(&matchID)
	require.NoError(t, err)

	nodes, _, err := s.GetMatchNodes(ctx, tournamentID, "Stage 1")
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "TeamA", nodes[0].Winner)
	assert.Equal(t, "3-1", nodes[0].Score)
}

func TestGetMatchNodes_NullableFormatHandled(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-null-format")
	seedMatch(t, tournamentID, "Stage 1", "TeamA", "TeamB", "pending")

	nodes, kind, err := s.GetMatchNodes(ctx, tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Equal(t, "", string(kind))
	assert.Len(t, nodes, 1)
}

func TestGetMatchNodes_OrdersChronologically(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-chronological-order", string(tournament.DoubleElim))
	nodes := []sources.MatchNode{
		{ID: "later", Team1: "TeamA", Team2: "TeamB", Status: "not_started", Timestamp: 2000},
		{ID: "earlier", Team1: "TeamC", Team2: "TeamD", Status: "not_started", Timestamp: 1000},
	}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.DoubleElim))

	got, _, err := s.GetMatchNodes(ctx, tournamentID, "Stage 1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "earlier", got[0].ID)
	assert.Equal(t, "later", got[1].ID)
}

// A finished match's completed_at (when the poller happened to notice it) and
// a pending match's scheduled_at (when it's slated to start) are different
// clocks. Ordering must use one consistent domain - scheduled_at - rather than
// completed_at for finished rows, or a later-scheduled but still-live match
// can sort ahead of one that already finished earlier in the schedule.
func TestGetMatchNodes_OrdersByScheduledTime_NotMixedWithCompletedTime(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-mixed-domain-order", string(tournament.DoubleElim))
	nodes := []sources.MatchNode{
		{ID: "finished-earlier", Team1: "TeamA", Team2: "TeamB", Status: "finished", Timestamp: 1000},
		{ID: "live-later", Team1: "TeamC", Team2: "TeamD", Status: "running", Timestamp: 2000},
	}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.DoubleElim))

	got, _, err := s.GetMatchNodes(ctx, tournamentID, "Stage 1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "finished-earlier", got[0].ID, "the match scheduled (and finished) first should sort first")
	assert.Equal(t, "live-later", got[1].ID)
}

func TestGetMatchNodes_OnlyReturnsRequestedRound(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-round-filter", "swiss")
	seedMatch(t, tournamentID, "Stage 1", "TeamA", "TeamB", "pending")
	seedMatch(t, tournamentID, "Stage 2", "TeamC", "TeamD", "pending")

	nodes, _, err := s.GetMatchNodes(ctx, tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "TeamA", nodes[0].Team1)
}
