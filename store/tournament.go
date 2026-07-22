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

// Tournament is a lightweight view of a tournament row, used to populate the
// /config set-tournament picklist. Source-agnostic: the caller stores the ID.
type Tournament struct {
	ID   int
	Name string
}

// ListTournaments returns all known tournaments ordered by name, for the
// /config set-tournament autocomplete. The tournaments table is populated
// independently (startup config + a separate ingestion script), so this is a
// plain read with no data-source coupling.
func (s *PostgresStore) ListTournaments(ctx context.Context) ([]Tournament, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM tournaments ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("ListTournaments: %w", err)
	}
	defer rows.Close()

	var tournaments []Tournament
	for rows.Next() {
		var t Tournament
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("ListTournaments: %w", err)
		}
		tournaments = append(tournaments, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListTournaments: %w", err)
	}
	return tournaments, nil
}
