package store

import (
	"context"
	"fmt"
)

// LeaderboardEntry represents a single user's score for a tournament.
type LeaderboardEntry struct {
	UserID    string
	Username  string
	Successes int
	Pending   int
	Failed    int
}

// GetLeaderboard returns all user scores for the given guild and tournament, ordered by successes descending.
func (s *PostgresStore) GetLeaderboard(ctx context.Context, guildID string, tournamentID int) ([]LeaderboardEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.user_id, u.username,
		       SUM(sc.successes) AS successes,
		       SUM(sc.pending)   AS pending,
		       SUM(sc.failed)    AS failed
		FROM scores sc
		JOIN predictions p ON p.id = sc.prediction_id
		JOIN users u ON u.user_id = p.user_id
		WHERE p.guild_id = $1 AND p.tournament_id = $2
		GROUP BY u.user_id, u.username
		ORDER BY successes DESC, pending DESC
	`, guildID, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("GetLeaderboard: %w", err)
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.Successes, &e.Pending, &e.Failed); err != nil {
			return nil, fmt.Errorf("GetLeaderboard: scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetLeaderboard: rows: %w", err)
	}
	return entries, nil
}
