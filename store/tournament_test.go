//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTournamentByExternalID_Success(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	// seedTournament inserts source="test", external_id="IEM Cologne 2025".
	want := seedTournament(t, "IEM Cologne 2025", "swiss")

	got, err := s.GetTournamentByExternalID(ctx, "test", "IEM Cologne 2025")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestGetTournamentByExternalID_FiltersBySource verifies the lookup keys on
// (source, external_id), not external_id alone — two sources can share an
// external_id (schema UNIQUE is on the pair), so the wrong source must not match.
func TestGetTournamentByExternalID_FiltersBySource(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	var pandaID int
	require.NoError(t, testPool.QueryRow(ctx,
		`INSERT INTO tournaments (external_id, source, name) VALUES ($1, 'pandascore', $2) RETURNING id`,
		"20710", "Major").Scan(&pandaID))
	_, err := testPool.Exec(ctx,
		`INSERT INTO tournaments (external_id, source, name) VALUES ($1, 'liquipedia', $2)`,
		"20710", "Major")
	require.NoError(t, err)

	got, err := s.GetTournamentByExternalID(ctx, "pandascore", "20710")
	require.NoError(t, err)
	assert.Equal(t, pandaID, got)
}

// TestGetTournamentByExternalID_NotFound guards the contract the app layer relies
// on: an unknown tournament surfaces pgx.ErrNoRows (via %w) so callers can tell
// "not configured yet" apart from a real DB error.
func TestGetTournamentByExternalID_NotFound(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	_, err := s.GetTournamentByExternalID(ctx, "pandascore", "does-not-exist")
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.Contains(t, err.Error(), "GetTournamentByExternalID")
}
