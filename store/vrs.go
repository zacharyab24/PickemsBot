package store

import (
	"context"
	"fmt"
	"time"
)

// VRSEntry represents a single team's entry in the VRS world rankings.
type VRSEntry struct {
	Standing      int
	Points        int
	TeamName      string
	Roster        []string
	StandingsDate time.Time
	SyncedAt      time.Time
}

// ListVRSRankings returns the latest VRS rankings for all teams, ordered by standing.
func (s *PostgresStore) ListVRSRankings(ctx context.Context) ([]VRSEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT canonical_name, standing, points, roster, standings_date, synced_at
		FROM (
			SELECT t.canonical_name, tr.standing, tr.points, tr.roster, tr.standings_date, tr.synced_at,
			       ROW_NUMBER() OVER (PARTITION BY tr.team_id ORDER BY tr.standings_date DESC) AS rn
			FROM team_rankings tr
			JOIN teams t ON t.id = tr.team_id
		) ranked
		WHERE rn = 1
		ORDER BY standing
	`)
	if err != nil {
		return nil, fmt.Errorf("ListVRSRankings: %w", err)
	}
	defer rows.Close()

	var results []VRSEntry
	for rows.Next() {
		var e VRSEntry
		if err := rows.Scan(&e.TeamName, &e.Standing, &e.Points, &e.Roster, &e.StandingsDate, &e.SyncedAt); err != nil {
			return nil, fmt.Errorf("ListVRSRankings: scan: %w", err)
		}
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListVRSRankings: rows: %w", err)
	}
	return results, nil
}

// SyncStandings replaces the stored VRS standings when the fetched snapshot is
// newer than what's stored. Returns true if a sync was performed, false if the
// DB already holds this snapshot's date. All entries come from one snapshot and
// share a StandingsDate; entries[0] supplies it. SyncedAt is stamped here.
//
// The clear-and-reinsert runs in a single transaction so the rankings are never
// observably empty (an old row's roster is preserved until the new set commits).
func (s *PostgresStore) SyncStandings(ctx context.Context, entries []VRSEntry) (bool, error) {
	if len(entries) == 0 {
		return false, fmt.Errorf("SyncStandings: no entries to sync")
	}
	fetchedDate := entries[0].StandingsDate

	needed, err := s.standingsSyncNeeded(ctx, fetchedDate)
	if err != nil {
		return false, err
	}
	if !needed {
		return false, nil
	}

	now := time.Now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("SyncStandings: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Clear old rankings; teams are preserved since team identities have value
	// beyond VRS data.
	if _, err := tx.Exec(ctx, `DELETE FROM team_rankings`); err != nil {
		return false, fmt.Errorf("SyncStandings: clear rankings: %w", err)
	}

	for _, e := range entries {
		var teamID int
		if err := tx.QueryRow(ctx, `
			INSERT INTO teams (canonical_name, source, external_id)
			VALUES ($1, 'vrs', $1)
			ON CONFLICT (source, external_id) DO UPDATE SET canonical_name = EXCLUDED.canonical_name
			RETURNING id
		`, e.TeamName).Scan(&teamID); err != nil {
			return false, fmt.Errorf("SyncStandings: upsert team %q: %w", e.TeamName, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO team_rankings (team_id, standing, points, roster, standings_date, synced_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (team_id, standings_date) DO UPDATE SET
				standing  = EXCLUDED.standing,
				points    = EXCLUDED.points,
				roster    = EXCLUDED.roster,
				synced_at = EXCLUDED.synced_at
		`, teamID, e.Standing, e.Points, e.Roster, fetchedDate, now); err != nil {
			return false, fmt.Errorf("SyncStandings: insert ranking for %q: %w", e.TeamName, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("SyncStandings: commit: %w", err)
	}
	return true, nil
}

// standingsSyncNeeded reports whether the fetched snapshot should replace what's
// stored: true when the table is empty, holds an inconsistent set of dates, or
// holds a different date than fetchedDate (compared at day granularity).
func (s *PostgresStore) standingsSyncNeeded(ctx context.Context, fetchedDate time.Time) (bool, error) {
	var groupCount int
	var latestDate *time.Time
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT standings_date), MAX(standings_date) FROM team_rankings
	`).Scan(&groupCount, &latestDate); err != nil {
		return false, fmt.Errorf("standingsSyncNeeded: %w", err)
	}

	switch groupCount {
	case 0:
		return true, nil
	case 1:
		return !latestDate.Truncate(24 * time.Hour).Equal(fetchedDate.Truncate(24 * time.Hour)), nil
	default:
		return true, nil // inconsistent state, resync
	}
}
