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