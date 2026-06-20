package store

import (
	"context"
	"fmt"
	"strconv"
)

// EnsureTournament upserts a tournament row by (source, external_id) and returns the internal SERIAL id.
// Format is left NULL on initial creation and gets set lazily when match nodes are first written.
func (s *PostgresStore) EnsureTournament(ctx context.Context, externalID, source, name string, seriesID int) (int, error) {
	var seriesIDStr *string
	if seriesID != 0 {
		v := strconv.Itoa(seriesID)
		seriesIDStr = &v
	}

	var id int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tournaments (external_id, source, name, series_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source, external_id) DO UPDATE
			SET name = EXCLUDED.name, series_id = COALESCE(EXCLUDED.series_id, tournaments.series_id)
		RETURNING id
	`, externalID, source, name, seriesIDStr).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("EnsureTournament: %w", err)
	}
	return id, nil
}
