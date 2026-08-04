//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListVRSRankings_Empty(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	entries, err := s.ListVRSRankings(ctx)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestListVRSRankings_ReturnsLatestPerTeam(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	teamID := seedTeam(t, "Cloud9", "vrs", "c9")

	olderDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newerDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	syncedAt := time.Now().UTC()

	_, err := testPool.Exec(ctx, `
		INSERT INTO team_rankings (team_id, standing, points, roster, standings_date, synced_at)
		VALUES ($1, 5, 800, ARRAY['PlayerA', 'PlayerB'], $2, $3)
	`, teamID, olderDate, syncedAt)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `
		INSERT INTO team_rankings (team_id, standing, points, roster, standings_date, synced_at)
		VALUES ($1, 3, 1200, ARRAY['PlayerC', 'PlayerD'], $2, $3)
	`, teamID, newerDate, syncedAt)
	require.NoError(t, err)

	entries, err := s.ListVRSRankings(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Cloud9", entries[0].TeamName)
	assert.Equal(t, 3, entries[0].Standing)
	assert.Equal(t, 1200, entries[0].Points)
}

func TestListVRSRankings_OrderedByStanding(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	teamNaVi := seedTeam(t, "NaVi", "vrs", "navi")
	teamFaze := seedTeam(t, "FaZe", "vrs", "faze")
	teamVitality := seedTeam(t, "Vitality", "vrs", "vitality")

	standingsDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	syncedAt := time.Now().UTC()

	_, err := testPool.Exec(ctx, `
		INSERT INTO team_rankings (team_id, standing, points, roster, standings_date, synced_at)
		VALUES ($1, 2, 1100, ARRAY['p1'], $2, $3)
	`, teamNaVi, standingsDate, syncedAt)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `
		INSERT INTO team_rankings (team_id, standing, points, roster, standings_date, synced_at)
		VALUES ($1, 1, 1500, ARRAY['p2'], $2, $3)
	`, teamFaze, standingsDate, syncedAt)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `
		INSERT INTO team_rankings (team_id, standing, points, roster, standings_date, synced_at)
		VALUES ($1, 3, 900, ARRAY['p3'], $2, $3)
	`, teamVitality, standingsDate, syncedAt)
	require.NoError(t, err)

	entries, err := s.ListVRSRankings(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	assert.Equal(t, 1, entries[0].Standing)
	assert.Equal(t, "FaZe", entries[0].TeamName)
	assert.Equal(t, 2, entries[1].Standing)
	assert.Equal(t, "NaVi", entries[1].TeamName)
	assert.Equal(t, 3, entries[2].Standing)
	assert.Equal(t, "Vitality", entries[2].TeamName)
}

func TestSyncStandings_InsertsAndReports(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	synced, err := s.SyncStandings(ctx, []VRSEntry{
		{Standing: 1, Points: 2000, TeamName: "Vitality", Roster: []string{"ZywOo", "apEX"}, StandingsDate: date},
		{Standing: 2, Points: 1800, TeamName: "FaZe", Roster: []string{"karrigan"}, StandingsDate: date},
	})
	require.NoError(t, err)
	assert.True(t, synced)

	got, err := s.ListVRSRankings(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Vitality", got[0].TeamName)
	assert.Equal(t, 1, got[0].Standing)
}

func TestSyncStandings_SkipsWhenDateUnchanged(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	entries := []VRSEntry{{Standing: 1, Points: 2000, TeamName: "Vitality", Roster: []string{"ZywOo"}, StandingsDate: date}}

	synced, err := s.SyncStandings(ctx, entries)
	require.NoError(t, err)
	require.True(t, synced)

	// Same snapshot date again is a no-op.
	synced, err = s.SyncStandings(ctx, entries)
	require.NoError(t, err)
	assert.False(t, synced)
}

func TestSyncStandings_ReplacesOnNewerDate(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	oldDate := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	_, err := s.SyncStandings(ctx, []VRSEntry{
		{Standing: 1, Points: 2000, TeamName: "Vitality", Roster: []string{"ZywOo"}, StandingsDate: oldDate},
		{Standing: 2, Points: 1800, TeamName: "FaZe", Roster: []string{"karrigan"}, StandingsDate: oldDate},
	})
	require.NoError(t, err)

	newDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	synced, err := s.SyncStandings(ctx, []VRSEntry{
		{Standing: 1, Points: 2100, TeamName: "Spirit", Roster: []string{"donk"}, StandingsDate: newDate},
	})
	require.NoError(t, err)
	require.True(t, synced)

	got, err := s.ListVRSRankings(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1) // old rankings cleared, teams preserved
	assert.Equal(t, "Spirit", got[0].TeamName)
}

func TestSyncStandings_EmptyEntriesError(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	_, err := s.SyncStandings(ctx, nil)
	assert.Error(t, err)
}
