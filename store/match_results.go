package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pickems-bot/sources"
	"pickems-bot/tournament"
)

// GetMatchResults derives a tournament.MatchResult from the stored match rows for the given tournament and round.
func (s *PostgresStore) GetMatchResults(ctx context.Context, tournamentID int, round string) (tournament.MatchResult, error) {
	nodes, kind, err := s.GetMatchNodes(ctx, tournamentID, round)
	if err != nil {
		return nil, fmt.Errorf("GetMatchResults: %w", err)
	}

	f, err := tournament.Get(kind)
	if err != nil {
		return nil, fmt.Errorf("GetMatchResults: unknown format %q: %w", kind, err)
	}

	result, err := f.BuildFromMatchNodes(nodes, round)
	if err != nil {
		return nil, fmt.Errorf("GetMatchResults: %w", err)
	}
	return result, nil
}

// UpsertMatchResults persists any match node updates implied by the result and materialises scores
// for all predictions on this tournament/round.
func (s *PostgresStore) UpsertMatchResults(ctx context.Context, tournamentID int, result tournament.MatchResult) error {
	if err := s.updateScores(ctx, tournamentID, result.GetRound(), result); err != nil {
		return fmt.Errorf("UpsertMatchResults: %w", err)
	}
	return nil
}

// FetchAndSaveMatchResults fetches match data from the configured data source, writes match rows,
// and materialises scores for all predictions on this tournament/round.
func (s *PostgresStore) FetchAndSaveMatchResults(ctx context.Context, tournamentID int, round string) error {
	result, nodes, err := s.fetcher.FetchMatchData(round)
	if err != nil {
		return fmt.Errorf("FetchAndSaveMatchResults: fetch: %w", err)
	}

	if result.GetType() == tournament.Swiss {
		nodes = tournament.NormalizeSwissSections(nodes)
	} else if result.GetType() == tournament.SingleElim {
		nodes = tournament.NormalizeSingleElimSections(nodes)
	}

	if err := s.upsertMatchNodes(ctx, tournamentID, round, nodes, result.GetType()); err != nil {
		return fmt.Errorf("FetchAndSaveMatchResults: %w", err)
	}

	if err := s.updateScores(ctx, tournamentID, round, result); err != nil {
		s.logger().Warn("FetchAndSaveMatchResults: score update failed", "error", err)
	}
	return nil
}

// upsertMatchNodes writes raw match node data into the matches table for a tournament/round.
func (s *PostgresStore) upsertMatchNodes(ctx context.Context, tournamentID int, round string, nodes []sources.MatchNode, kind tournament.Kind) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("upsertMatchNodes: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, n := range nodes {
		winner := nullString(n.Winner)
		score := nullString(n.Score)
		extID := nullString(n.ID)
		section := nullString(n.Section)

		status := "pending"
		switch n.Status {
		case "finished":
			status = "completed"
		case "running":
			status = "in_progress"
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO matches (tournament_id, round, section, team1_name, team2_name, score, external_id, status, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (tournament_id, external_id) WHERE external_id IS NOT NULL
			DO UPDATE SET
				round        = EXCLUDED.round,
				team1_name   = EXCLUDED.team1_name,
				team2_name   = EXCLUDED.team2_name,
				section      = EXCLUDED.section,
				score        = EXCLUDED.score,
				status       = EXCLUDED.status,
				completed_at = EXCLUDED.completed_at
		`, tournamentID, round, section, n.Team1, n.Team2, score, extID, status,
			completedAt(status))
		if err != nil {
			return fmt.Errorf("upsertMatchNodes: insert %q vs %q: %w", n.Team1, n.Team2, err)
		}

		// Update winner_id FK if we have a winner name and can resolve it to a team row.
		if winner != nil {
			_, err = tx.Exec(ctx, `
				UPDATE matches SET winner_id = (
					SELECT id FROM teams WHERE canonical_name = $1 LIMIT 1
				)
				WHERE tournament_id = $2 AND external_id = $3 AND external_id IS NOT NULL
			`, *winner, tournamentID, n.ID)
			if err != nil {
				s.logger().Warn("upsertMatchNodes: could not resolve winner FK", "team", *winner, "error", err)
			}
		}
	}

	// Upsert all participant team names into the teams table so that pick inserts
	// can resolve team_id FKs. Uses the match node team name as both canonical_name
	// and external_id within the 'pandascore' source namespace.
	seen := make(map[string]bool)
	for _, n := range nodes {
		for _, name := range []string{n.Team1, n.Team2} {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			if _, err := tx.Exec(ctx, `
				INSERT INTO teams (canonical_name, source, external_id)
				VALUES ($1, 'pandascore', $1)
				ON CONFLICT (source, external_id) DO UPDATE SET canonical_name = EXCLUDED.canonical_name
			`, name); err != nil {
				return fmt.Errorf("upsertMatchNodes: ensure team %q: %w", name, err)
			}
		}
	}

	// Resolve team1_id / team2_id FKs on all matches for this tournament/round.
	if _, err := tx.Exec(ctx, `
		UPDATE matches m
		SET
			team1_id = (SELECT id FROM teams WHERE canonical_name = m.team1_name AND source = 'pandascore' LIMIT 1),
			team2_id = (SELECT id FROM teams WHERE canonical_name = m.team2_name AND source = 'pandascore' LIMIT 1)
		WHERE tournament_id = $1 AND round = $2
	`, tournamentID, round); err != nil {
		return fmt.Errorf("upsertMatchNodes: set team FKs: %w", err)
	}

	// Set tournament format lazily on first result write (format is NULL until match data arrives).
	if _, err := tx.Exec(ctx, `
		UPDATE tournaments SET format = $1 WHERE id = $2 AND format IS NULL
	`, string(kind), tournamentID); err != nil {
		return fmt.Errorf("upsertMatchNodes: set format: %w", err)
	}

	return tx.Commit(ctx)
}

// updateScores materialises scores for all predictions on this tournament/round based on the current result.
// Called after match data is written so that leaderboard reads are always up to date.
func (s *PostgresStore) updateScores(ctx context.Context, tournamentID int, round string, result tournament.MatchResult) error {
	f, err := tournament.Get(result.GetType())
	if err != nil {
		return fmt.Errorf("updateScores: unknown format: %w", err)
	}

	predictions, err := s.ListPredictions(ctx, "", tournamentID, round)
	if err != nil {
		return fmt.Errorf("updateScores: list predictions: %w", err)
	}

	for _, p := range predictions {
		report, err := f.CalculateScore(p, result)
		if err != nil {
			s.logger().Warn("updateScores: score calculation failed", "user", p.UserID, "error", err)
			continue
		}
		score := report.GetScore()

		if _, err := s.pool.Exec(ctx, `
			INSERT INTO scores (prediction_id, successes, pending, failed, last_computed_at)
			SELECT p.id, $2, $3, $4, NOW()
			FROM predictions p
			WHERE p.user_id = $1 AND p.tournament_id = $5 AND p.round = $6
			ON CONFLICT (prediction_id) DO UPDATE SET
				successes        = EXCLUDED.successes,
				pending          = EXCLUDED.pending,
				failed           = EXCLUDED.failed,
				last_computed_at = NOW()
		`, p.UserID, score.Successes, score.Pending, score.Failed, tournamentID, round); err != nil {
			s.logger().Warn("updateScores: upsert failed", "user", p.UserID, "error", err)
		}
	}
	return nil
}

// completedAt returns the current UTC time when a match is completed, nil otherwise.
func completedAt(status string) *time.Time {
	if status == "completed" {
		t := time.Now().UTC()
		return &t
	}
	return nil
}
