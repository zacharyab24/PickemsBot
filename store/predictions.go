package store

import (
	"context"
	"fmt"
	"strings"

	"pickems-bot/models"
	"pickems-bot/tournament"

	"github.com/jackc/pgx/v5"
)

// UpsertPrediction inserts or replaces a user's prediction for a tournament round.
// Picks are replaced wholesale — existing rows are deleted before new ones are inserted.
func (s *PostgresStore) UpsertPrediction(ctx context.Context, guildID string, tournamentID int, prediction models.Prediction) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("UpsertPrediction: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Ensure guild and user rows exist.
	if _, err := tx.Exec(ctx, `INSERT INTO guilds (guild_id) VALUES ($1) ON CONFLICT DO NOTHING`, guildID); err != nil {
		return fmt.Errorf("UpsertPrediction: ensure guild: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (user_id, username) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET username = EXCLUDED.username
	`, prediction.UserID, prediction.Username); err != nil {
		return fmt.Errorf("UpsertPrediction: ensure user: %w", err)
	}

	// Upsert the prediction header row and get its ID.
	var predictionID int
	err = tx.QueryRow(ctx, `
		INSERT INTO predictions (user_id, guild_id, tournament_id, round, format, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id, guild_id, tournament_id, round)
		DO UPDATE SET format = EXCLUDED.format, updated_at = NOW()
		RETURNING id
	`, prediction.UserID, guildID, tournamentID, prediction.Round, prediction.Format).Scan(&predictionID)
	if err != nil {
		return fmt.Errorf("UpsertPrediction: upsert prediction row: %w", err)
	}

	// Replace picks for this prediction.
	if _, err := tx.Exec(ctx, `DELETE FROM swiss_picks WHERE prediction_id = $1`, predictionID); err != nil {
		return fmt.Errorf("UpsertPrediction: clear swiss picks: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM single_elim_picks WHERE prediction_id = $1`, predictionID); err != nil {
		return fmt.Errorf("UpsertPrediction: clear elim picks: %w", err)
	}

	switch tournament.Kind(prediction.Format) {
	case tournament.Swiss:
		for _, team := range prediction.Win {
			if err := insertSwissPick(ctx, tx, predictionID, team, "win"); err != nil {
				return fmt.Errorf("UpsertPrediction: %w", err)
			}
		}
		for _, team := range prediction.Advance {
			if err := insertSwissPick(ctx, tx, predictionID, team, "advance"); err != nil {
				return fmt.Errorf("UpsertPrediction: %w", err)
			}
		}
		for _, team := range prediction.Lose {
			if err := insertSwissPick(ctx, tx, predictionID, team, "lose"); err != nil {
				return fmt.Errorf("UpsertPrediction: %w", err)
			}
		}
	case tournament.SingleElim:
		for team, prog := range prediction.Progression {
			if err := insertElimPick(ctx, tx, predictionID, team, prog.Round, prog.Status); err != nil {
				return fmt.Errorf("UpsertPrediction: %w", err)
			}
		}
	default:
		return fmt.Errorf("UpsertPrediction: unknown format %q", prediction.Format)
	}

	return tx.Commit(ctx)
}

// GetPrediction retrieves the stored prediction for the given user, guild, tournament, and round.
func (s *PostgresStore) GetPrediction(ctx context.Context, userID, guildID string, tournamentID int, round string) (models.Prediction, error) {
	return s.fetchPrediction(ctx, `
		SELECT p.id, p.user_id, u.username, p.round, p.format
		FROM predictions p
		JOIN users u ON u.user_id = p.user_id
		WHERE p.user_id = $1 AND p.guild_id = $2 AND p.tournament_id = $3 AND p.round = $4
	`, userID, guildID, tournamentID, round)
}

// GetPredictionByUsername retrieves the stored prediction for the given username (case-insensitive).
func (s *PostgresStore) GetPredictionByUsername(ctx context.Context, username, guildID string, tournamentID int, round string) (models.Prediction, error) {
	return s.fetchPrediction(ctx, `
		SELECT p.id, p.user_id, u.username, p.round, p.format
		FROM predictions p
		JOIN users u ON u.user_id = p.user_id
		WHERE LOWER(u.username) = LOWER($1) AND p.guild_id = $2 AND p.tournament_id = $3 AND p.round = $4
	`, username, guildID, tournamentID, round)
}

// ListPredictions returns all stored predictions for a tournament and round.
// Pass an empty guildID to return predictions across all guilds (used by score materialisation).
func (s *PostgresStore) ListPredictions(ctx context.Context, guildID string, tournamentID int, round string) ([]models.Prediction, error) {
	query := `
		SELECT p.id, p.user_id, u.username, p.round, p.format
		FROM predictions p
		JOIN users u ON u.user_id = p.user_id
		WHERE p.tournament_id = $1 AND p.round = $2
	`
	args := []any{tournamentID, round}
	if guildID != "" {
		query += ` AND p.guild_id = $3`
		args = append(args, guildID)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListPredictions: %w", err)
	}
	defer rows.Close()

	var predictions []models.Prediction
	for rows.Next() {
		var predID int
		var p models.Prediction
		if err := rows.Scan(&predID, &p.UserID, &p.Username, &p.Round, &p.Format); err != nil {
			return nil, fmt.Errorf("ListPredictions: scan: %w", err)
		}
		if err := s.populatePicks(ctx, predID, &p); err != nil {
			return nil, fmt.Errorf("ListPredictions: %w", err)
		}
		predictions = append(predictions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListPredictions: rows: %w", err)
	}
	return predictions, nil
}

// ListValidTeams returns the distinct team names for all matches in a tournament round, and the tournament format.
// All matches in the round are included regardless of status — teams from completed matches are still valid picks.
func (s *PostgresStore) ListValidTeams(ctx context.Context, tournamentID int, round string) ([]string, tournament.Kind, error) {
	var format string
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(format, '') FROM tournaments WHERE id = $1`, tournamentID).Scan(&format)
	if err != nil {
		return nil, "", fmt.Errorf("ListValidTeams: fetch format: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT name FROM (
			SELECT COALESCE(t1.canonical_name, m.team1_name) AS name FROM matches m LEFT JOIN teams t1 ON t1.id = m.team1_id WHERE m.tournament_id = $1 AND m.round = $2
			UNION
			SELECT COALESCE(t2.canonical_name, m.team2_name) AS name FROM matches m LEFT JOIN teams t2 ON t2.id = m.team2_id WHERE m.tournament_id = $1 AND m.round = $2
		) names
		WHERE name IS NOT NULL AND name != ''
		ORDER BY name
	`, tournamentID, round)
	if err != nil {
		return nil, "", fmt.Errorf("ListValidTeams: %w", err)
	}
	defer rows.Close()

	var teams []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, "", fmt.Errorf("ListValidTeams: scan: %w", err)
		}
		teams = append(teams, name)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("ListValidTeams: rows: %w", err)
	}
	return teams, tournament.Kind(format), nil
}

// fetchPrediction is a shared helper for single-prediction reads.
func (s *PostgresStore) fetchPrediction(ctx context.Context, query string, args ...any) (models.Prediction, error) {
	var predID int
	var p models.Prediction
	err := s.pool.QueryRow(ctx, query, args...).Scan(&predID, &p.UserID, &p.Username, &p.Round, &p.Format)
	if err != nil {
		return models.Prediction{}, fmt.Errorf("fetchPrediction: %w", err)
	}
	if err := s.populatePicks(ctx, predID, &p); err != nil {
		return models.Prediction{}, err
	}
	return p, nil
}

// populatePicks loads format-specific picks into a Prediction based on its Format field.
func (s *PostgresStore) populatePicks(ctx context.Context, predictionID int, p *models.Prediction) error {
	switch tournament.Kind(p.Format) {
	case tournament.Swiss:
		rows, err := s.pool.Query(ctx, `
			SELECT t.canonical_name, sp.bucket
			FROM swiss_picks sp
			JOIN teams t ON t.id = sp.team_id
			WHERE sp.prediction_id = $1
		`, predictionID)
		if err != nil {
			return fmt.Errorf("populatePicks: swiss: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name, bucket string
			if err := rows.Scan(&name, &bucket); err != nil {
				return fmt.Errorf("populatePicks: swiss scan: %w", err)
			}
			switch bucket {
			case "win":
				p.Win = append(p.Win, name)
			case "advance":
				p.Advance = append(p.Advance, name)
			case "lose":
				p.Lose = append(p.Lose, name)
			}
		}
		return rows.Err()

	case tournament.SingleElim:
		rows, err := s.pool.Query(ctx, `
			SELECT t.canonical_name, ep.predicted_round, ep.predicted_status
			FROM single_elim_picks ep
			JOIN teams t ON t.id = ep.team_id
			WHERE ep.prediction_id = $1
		`, predictionID)
		if err != nil {
			return fmt.Errorf("populatePicks: elim: %w", err)
		}
		defer rows.Close()
		p.Progression = make(map[string]models.TeamProgress)
		for rows.Next() {
			var name, round, status string
			if err := rows.Scan(&name, &round, &status); err != nil {
				return fmt.Errorf("populatePicks: elim scan: %w", err)
			}
			p.Progression[name] = models.TeamProgress{Round: round, Status: status}
		}
		return rows.Err()

	default:
		return fmt.Errorf("populatePicks: unknown format %q", p.Format)
	}
}

// insertSwissPick inserts a single Swiss pick, resolving the team name to a teams.id FK.
func insertSwissPick(ctx context.Context, tx pgx.Tx, predictionID int, teamName, bucket string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO swiss_picks (prediction_id, team_id, bucket)
		SELECT $1, id, $3 FROM teams WHERE canonical_name = $2 LIMIT 1
	`, predictionID, strings.TrimSpace(teamName), bucket)
	if err != nil {
		return fmt.Errorf("insertSwissPick %q: %w", teamName, err)
	}
	return nil
}

// insertElimPick inserts a single single-elimination pick, resolving the team name to a teams.id FK.
func insertElimPick(ctx context.Context, tx pgx.Tx, predictionID int, teamName, predictedRound, predictedStatus string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO single_elim_picks (prediction_id, team_id, predicted_round, predicted_status)
		SELECT $1, id, $3, $4 FROM teams WHERE canonical_name = $2 LIMIT 1
	`, predictionID, strings.TrimSpace(teamName), predictedRound, predictedStatus)
	if err != nil {
		return fmt.Errorf("insertElimPick %q: %w", teamName, err)
	}
	return nil
}
