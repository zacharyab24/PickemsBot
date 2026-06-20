package store

import (
	"context"
	"fmt"
)

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
