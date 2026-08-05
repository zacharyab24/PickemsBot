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

// TournamentCatalogEntry is one resolved PandaScore tournament (stage) to upsert
// into the catalog. The caller (the ingest job) has already turned the raw
// PandaScore payload into these fields, so the store stays free of source types.
type TournamentCatalogEntry struct {
	ExternalID string
	Name       string
	Round      string
	SeriesID   string
}

// SyncTournaments upserts the active (upcoming + running) PandaScore tournaments
// as not-finished, then marks any pandascore row that has dropped out of the
// active set as finished. Both steps run in one transaction.
//
// The finished-sweep only runs when active is non-empty: an empty set almost
// always means a bad API response rather than "no tournaments exist", and
// sweeping on it would wrongly mark every row finished. Callers must ensure both
// PandaScore fetches succeeded before combining them into active; a failed fetch
// must not reach here as a short active set. An empty active set is rejected.
//
// Unlike EnsureTournament (the startup path, which preserves round on conflict),
// this owns round/series_id for pandascore catalog rows and overwrites them, and
// resets is_finished to false: anything in the active set is by definition live,
// so a previously-swept tournament that reappears is reactivated.
//
// Returns the internal DB ids of any row the finished-sweep just flipped to
// finished, so the caller (ingest.TournamentSync) can drop them from the
// poller's monitoring pool - a tournament already finished before this run
// isn't included, since it doesn't transition here and this call has no way
// to tell that apart from one that's been finished all along.
func (s *PostgresStore) SyncTournaments(ctx context.Context, active []TournamentCatalogEntry) ([]int, error) {
	if len(active) == 0 {
		return nil, fmt.Errorf("SyncTournaments: empty active set, refusing to run finished-sweep")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("SyncTournaments: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	activeIDs := make([]string, 0, len(active))
	for _, t := range active {
		activeIDs = append(activeIDs, t.ExternalID)

		if _, err := tx.Exec(ctx, `
			INSERT INTO tournaments (external_id, source, name, round, series_id, is_finished)
			VALUES ($1, 'pandascore', $2, $3, $4, false)
			ON CONFLICT (source, external_id) DO UPDATE SET
				name        = EXCLUDED.name,
				round       = EXCLUDED.round,
				series_id   = EXCLUDED.series_id,
				is_finished = false
		`, t.ExternalID, t.Name, t.Round, t.SeriesID); err != nil {
			return nil, fmt.Errorf("SyncTournaments: upsert %q: %w", t.Name, err)
		}
	}

	rows, err := tx.Query(ctx, `
		UPDATE tournaments
		   SET is_finished = true
		 WHERE source = 'pandascore'
		   AND is_finished = false
		   AND external_id <> ALL($1)
		RETURNING id
	`, activeIDs)
	if err != nil {
		return nil, fmt.Errorf("SyncTournaments: finished-sweep: %w", err)
	}
	var newlyFinished []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("SyncTournaments: finished-sweep scan: %w", err)
		}
		newlyFinished = append(newlyFinished, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("SyncTournaments: finished-sweep: %w", err)
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("SyncTournaments: commit: %w", err)
	}
	return newlyFinished, nil
}

// Tournament is a lightweight view of a tournament row, used to populate the
// /config set-tournament picklist. Source-agnostic: the caller stores the ID.
// Round is the stage label (e.g. "Qualifier", "Playoffs") that distinguishes
// rows sharing a name within a series; it may be empty for sources that don't
// split a tournament into stages.
type Tournament struct {
	ID         int
	Source     string
	ExternalID string
	Round      string
	Name       string
	SeriesID   string
	Format     *string
	IsFinished bool
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
		tournamentSelect+`
		 WHERE name = $1 AND COALESCE(round, '') = $2
		 ORDER BY id LIMIT 1`, name, round,
	).Scan(&t.ID, &t.Source, &t.ExternalID, &t.Round, &t.Name, &t.SeriesID, &t.Format, &t.IsFinished)
	if err != nil {
		return Tournament{}, fmt.Errorf("GetTournamentByNameAndRound: %w", err)
	}
	return t, nil
}

// GetTournament returns a single tournament by its internal DB id. Used by
// /config set-round to recover the currently-configured tournament's name so
// the round autocomplete can be scoped to that tournament, and by the format
// detection hook to read source/external_id/format.
func (s *PostgresStore) GetTournament(ctx context.Context, id int) (Tournament, error) {
	var t Tournament
	err := s.pool.QueryRow(ctx,
		tournamentSelect+` WHERE id = $1`, id,
	).Scan(&t.ID, &t.Source, &t.ExternalID, &t.Round, &t.Name, &t.SeriesID, &t.Format, &t.IsFinished)
	if err != nil {
		return Tournament{}, fmt.Errorf("GetTournament: %w", err)
	}
	return t, nil
}

// tournamentSelect is the shared column list for reading a full Tournament row.
// Nullable text columns are COALESCEd to ""; format stays nullable (*string) so
// callers can tell "not yet detected" (nil) from a real value.
const tournamentSelect = `SELECT id, source, external_id, COALESCE(round, ''), name, COALESCE(series_id, ''), format, is_finished FROM tournaments`

// SetTournamentFormat records the detected format for a tournament. The caller
// decides when to run detection (e.g. only when format is currently unset); this
// is a plain setter and will overwrite any existing value.
func (s *PostgresStore) SetTournamentFormat(ctx context.Context, id int, format string) error {
	if _, err := s.pool.Exec(ctx,
		`UPDATE tournaments SET format = $1 WHERE id = $2`, format, id); err != nil {
		return fmt.Errorf("SetTournamentFormat: %w", err)
	}
	return nil
}
