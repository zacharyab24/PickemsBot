package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"pickems-bot/sources"
)

// EnsureScheduledMatches verifies that at least one pending match exists in the database for the given tournament.
// Prediction operations depend on this data being present, so callers should use this as a precondition check.
func (s *PostgresStore) EnsureScheduledMatches(ctx context.Context, tournamentID int) error {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM matches
		WHERE tournament_id = $1 AND status = 'pending'
	`, tournamentID).Scan(&count)
	if err != nil {
		return fmt.Errorf("EnsureScheduledMatches: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no scheduled matches found for tournament %d", tournamentID)
	}
	return nil
}

// GetMatchSchedule returns all pending matches for the given tournament, ordered by scheduled time.
func (s *PostgresStore) GetMatchSchedule(ctx context.Context, tournamentID int) ([]sources.ScheduledMatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT team1_name, team2_name, scheduled_at, best_of, stream_url, is_live,
		       (status = 'completed') AS finished
		FROM matches
		WHERE tournament_id = $1 AND status != 'completed' AND best_of IS NOT NULL
		ORDER BY scheduled_at ASC
	`, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("GetMatchSchedule: %w", err)
	}
	defer rows.Close()

	var matches []sources.ScheduledMatch
	for rows.Next() {
		var m sources.ScheduledMatch
		var scheduledAt *time.Time
		var bestOf, streamURL *string
		if err := rows.Scan(&m.Team1, &m.Team2, &scheduledAt, &bestOf, &streamURL, &m.Live, &m.Finished); err != nil {
			return nil, fmt.Errorf("GetMatchSchedule: scan: %w", err)
		}
		if scheduledAt != nil {
			m.EpochTime = scheduledAt.Unix()
		}
		if bestOf != nil {
			m.BestOf = *bestOf
		}
		if streamURL != nil {
			m.StreamURL = *streamURL
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetMatchSchedule: rows: %w", err)
	}
	return matches, nil
}

// UpsertMatchSchedule replaces all pending non-live matches for the tournament with the provided list.
// Completed and in-progress matches are preserved.
func (s *PostgresStore) UpsertMatchSchedule(ctx context.Context, tournamentID int, matches []sources.ScheduledMatch) error {
	if len(matches) == 0 {
		return fmt.Errorf("UpsertMatchSchedule: matches list is empty")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("UpsertMatchSchedule: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		DELETE FROM matches
		WHERE tournament_id = $1 AND status != 'completed'
	`, tournamentID)
	if err != nil {
		return fmt.Errorf("UpsertMatchSchedule: clear pending: %w", err)
	}

	for _, m := range matches {
		scheduledAt := time.Unix(m.EpochTime, 0).UTC()
		status := "pending"
		if m.Finished {
			status = "completed"
		} else if m.Live {
			status = "in_progress"
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO matches (tournament_id, round, team1_name, team2_name, scheduled_at, best_of, stream_url, is_live, status)
			VALUES ($1, '', $2, $3, $4, $5, $6, $7, $8)
		`, tournamentID, m.Team1, m.Team2, scheduledAt, nullString(m.BestOf), nullString(m.StreamURL), m.Live, status)
		if err != nil {
			return fmt.Errorf("UpsertMatchSchedule: insert: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("UpsertMatchSchedule: commit: %w", err)
	}
	return nil
}

// FetchAndSaveSchedule fetches upcoming matches from the configured data source and persists them for the given tournament.
func (s *PostgresStore) FetchAndSaveSchedule(ctx context.Context, tournamentID int) error {
	matches, err := s.fetcher.FetchSchedule()
	if err != nil {
		return fmt.Errorf("FetchAndSaveSchedule: %w", err)
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].EpochTime < matches[j].EpochTime
	})
	return s.UpsertMatchSchedule(ctx, tournamentID, matches)
}

// nullString converts an empty string to nil for nullable TEXT columns.
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
