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
// Round is the stage label (e.g. "Qualifier", "Playoffs") that distinguishes
// rows sharing a name within a series; it may be empty for sources that don't
// split a tournament into stages.
type Tournament struct {
	ID    int
	Name  string
	Round string
}

// ListTournamentNames returns the distinct tournament names for the /config
// set-tournament autocomplete. A single name (e.g. "BLAST Bounty Summer 2026")
// can have several rows - one per round/stage - so the picklist collapses them
// to one entry; the round is chosen separately via ListRoundsForTournament.
// The tournaments table is populated independently (startup config + a separate
// ingestion script), so this is a plain read with no data-source coupling.
func (s *PostgresStore) ListTournamentNames(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT name FROM tournaments ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("ListTournamentNames: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("ListTournamentNames: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListTournamentNames: %w", err)
	}
	return names, nil
}

// ListRoundsForTournament returns the distinct non-empty rounds recorded for a
// tournament name, for the round autocomplete on /config set-tournament and
// /config set-round. Rows with no round are skipped - there's nothing to pick.
func (s *PostgresStore) ListRoundsForTournament(ctx context.Context, name string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT round FROM tournaments WHERE name = $1 AND round IS NOT NULL AND round <> '' ORDER BY round`, name)
	if err != nil {
		return nil, fmt.Errorf("ListRoundsForTournament: %w", err)
	}
	defer rows.Close()

	var rounds []string
	for rows.Next() {
		var round string
		if err := rows.Scan(&round); err != nil {
			return nil, fmt.Errorf("ListRoundsForTournament: %w", err)
		}
		rounds = append(rounds, round)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListRoundsForTournament: %w", err)
	}
	return rounds, nil
}

// GetTournamentByNameAndRound resolves a (name, round) pair - the two values the
// admin picks in /config - to the specific tournament row, so the caller can
// store its internal id. round is nullable in the schema; COALESCE lets an empty
// round match a NULL row for single-stage tournaments. If more than one row
// matches (e.g. the same tournament ingested from two sources) the lowest id
// wins, keeping the result deterministic.
func (s *PostgresStore) GetTournamentByNameAndRound(ctx context.Context, name, round string) (Tournament, error) {
	var t Tournament
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(round, '') FROM tournaments
		 WHERE name = $1 AND COALESCE(round, '') = $2
		 ORDER BY id LIMIT 1`, name, round,
	).Scan(&t.ID, &t.Name, &t.Round)
	if err != nil {
		return Tournament{}, fmt.Errorf("GetTournamentByNameAndRound: %w", err)
	}
	return t, nil
}

// GetTournament returns a single tournament by its internal DB id. Used by
// /config set-round to recover the currently-configured tournament's name so
// the round autocomplete can be scoped to that tournament.
func (s *PostgresStore) GetTournament(ctx context.Context, id int) (Tournament, error) {
	var t Tournament
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(round, '') FROM tournaments WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Round)
	if err != nil {
		return Tournament{}, fmt.Errorf("GetTournament: %w", err)
	}
	return t, nil
}
