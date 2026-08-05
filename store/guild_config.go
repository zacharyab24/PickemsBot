package store

import (
	"context"
	"fmt"
)

// GuildConfig holds the per-guild tournament configuration stored in the guild_config table.
type GuildConfig struct {
	ID                    int
	GuildID               string
	TournamentID          *int
	TournamentName        *string
	Round                 *string
	Format                *string
	ResultsChannelID      *string
	NotificationChannelID *string
}

// GetGuildConfig retrieves the guild config for the given guild and results channel,
// including tournament name and format from a LEFT JOIN on tournaments.
func (s *PostgresStore) GetGuildConfig(ctx context.Context, guildID, channelID string) (GuildConfig, error) {
	var cfg GuildConfig
	err := s.pool.QueryRow(ctx, `
		SELECT gc.id, gc.guild_id, gc.tournament_id, gc.round, gc.results_channel_id, gc.notification_channel_id,
		       t.name, t.format
		FROM guild_config gc
		LEFT JOIN tournaments t ON t.id = gc.tournament_id
		WHERE gc.guild_id = $1 AND gc.results_channel_id = $2
	`, guildID, channelID).Scan(
		&cfg.ID, &cfg.GuildID, &cfg.TournamentID, &cfg.Round,
		&cfg.ResultsChannelID, &cfg.NotificationChannelID,
		&cfg.TournamentName, &cfg.Format,
	)
	if err != nil {
		return GuildConfig{}, fmt.Errorf("GetGuildConfig: %w", err)
	}
	return cfg, nil
}

// UpsertGuildConfig upserts a guild config. If a config for the guild and results channel already exists it is updated, otherwise a new row is inserted.
func (s *PostgresStore) UpsertGuildConfig(ctx context.Context, cfg GuildConfig) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO guild_config (guild_id, tournament_id, round, results_channel_id, notification_channel_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (guild_id, results_channel_id)
		DO UPDATE SET
			tournament_id           = EXCLUDED.tournament_id,
			round                   = EXCLUDED.round,
			notification_channel_id = EXCLUDED.notification_channel_id
	`, cfg.GuildID, cfg.TournamentID, cfg.Round, cfg.ResultsChannelID, cfg.NotificationChannelID)
	if err != nil {
		return fmt.Errorf("SetGuildConfig: %w", err)
	}
	return nil
}

// EnsureGuild inserts a guilds row if one does not already exist.
// Must be called before UpsertGuildConfig — guild_config.guild_id has an FK to guilds.
func (s *PostgresStore) EnsureGuild(ctx context.Context, guildID string) error {
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO guilds (guild_id) VALUES ($1) ON CONFLICT DO NOTHING`, guildID); err != nil {
		return fmt.Errorf("EnsureGuild: %w", err)
	}
	return nil
}

// ListTrackedTournamentIDs returns the distinct tournament ids referenced
// across every guild_config row. Used once at startup to re-seed the poller's
// monitoring pool after a restart, since the pool itself starts empty and has
// no memory of what was being tracked before the process stopped.
func (s *PostgresStore) ListTrackedTournamentIDs(ctx context.Context) ([]int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT tournament_id FROM guild_config WHERE tournament_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("ListTrackedTournamentIDs: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ListTrackedTournamentIDs: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListTrackedTournamentIDs: %w", err)
	}
	return ids, nil
}

// TournamentStillReferenced reports whether any guild_config row other than
// excludeConfigID still points at tournamentID. Used before dropping a
// tournament from the poller's monitoring pool when a guild switches away
// from it, so a tournament another guild is still tracking isn't stopped.
func (s *PostgresStore) TournamentStillReferenced(ctx context.Context, tournamentID, excludeConfigID int) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM guild_config WHERE tournament_id = $1 AND id <> $2)`,
		tournamentID, excludeConfigID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("TournamentStillReferenced: %w", err)
	}
	return exists, nil
}
