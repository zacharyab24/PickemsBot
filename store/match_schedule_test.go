//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"pickems-bot/sources"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertMatchSchedule_InsertsRows(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-schedule-insert", "swiss")

	matches := []sources.ScheduledMatch{
		{Team1: "TeamA", Team2: "TeamB", BestOf: "3", EpochTime: time.Now().Add(1 * time.Hour).Unix()},
		{Team1: "TeamC", Team2: "TeamD", BestOf: "3", EpochTime: time.Now().Add(2 * time.Hour).Unix()},
	}

	err := s.UpsertMatchSchedule(ctx, tournamentID, matches)
	require.NoError(t, err)

	got, err := s.GetMatchSchedule(ctx, tournamentID)
	require.NoError(t, err)
	assert.Len(t, got, 2)

	team1s := []string{got[0].Team1, got[1].Team1}
	assert.Contains(t, team1s, "TeamA")
	assert.Contains(t, team1s, "TeamC")
}

func TestUpsertMatchSchedule_Idempotent(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-schedule-idempotent", "swiss")

	matches := []sources.ScheduledMatch{
		{Team1: "TeamA", Team2: "TeamB", BestOf: "3", EpochTime: time.Now().Add(1 * time.Hour).Unix()},
		{Team1: "TeamC", Team2: "TeamD", BestOf: "3", EpochTime: time.Now().Add(2 * time.Hour).Unix()},
	}

	require.NoError(t, s.UpsertMatchSchedule(ctx, tournamentID, matches))
	require.NoError(t, s.UpsertMatchSchedule(ctx, tournamentID, matches))

	got, err := s.GetMatchSchedule(ctx, tournamentID)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestGetMatchSchedule_Empty(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-schedule-empty", "swiss")

	got, err := s.GetMatchSchedule(ctx, tournamentID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestEnsureScheduledMatches_NoMatches(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-ensure-empty", "swiss")

	err := s.EnsureScheduledMatches(ctx, tournamentID)
	assert.Error(t, err)
}

func TestEnsureScheduledMatches_WithPendingMatches(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-ensure-pending", "swiss")
	seedMatch(t, tournamentID, "Stage 1", "TeamA", "TeamB", "pending")

	err := s.EnsureScheduledMatches(ctx, tournamentID)
	assert.NoError(t, err)
}

func TestEnsureScheduledMatches_AllCompleted(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-ensure-completed", "swiss")
	seedMatch(t, tournamentID, "Stage 1", "TeamA", "TeamB", "completed")
	seedMatch(t, tournamentID, "Stage 1", "TeamC", "TeamD", "completed")

	err := s.EnsureScheduledMatches(ctx, tournamentID)
	assert.NoError(t, err)
}
