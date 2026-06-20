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
