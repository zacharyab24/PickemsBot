//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertGuildConfig_Insert(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	seedGuild(t, "guild-1")

	round := "Stage 1"
	channel := "channel-1"
	cfg := GuildConfig{
		GuildID:          "guild-1",
		Round:            &round,
		ResultsChannelID: &channel,
	}
	err := s.UpsertGuildConfig(ctx, cfg)
	assert.NoError(t, err)
}

func TestUpsertGuildConfig_Update(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	seedGuild(t, "guild-1")

	channel := "channel-1"
	round1 := "Stage 1"
	cfg := GuildConfig{
		GuildID:          "guild-1",
		Round:            &round1,
		ResultsChannelID: &channel,
	}
	require.NoError(t, s.UpsertGuildConfig(ctx, cfg))

	round2 := "Stage 2"
	cfg.Round = &round2
	require.NoError(t, s.UpsertGuildConfig(ctx, cfg))

	got, err := s.GetGuildConfig(ctx, "guild-1", "channel-1")
	require.NoError(t, err)
	assert.Equal(t, "Stage 2", *got.Round)
}

func TestGetGuildConfig_Success(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	seedGuild(t, "guild-1")

	channel := "channel-1"
	notif := "notif-1"
	round := "Stage 1"
	cfg := GuildConfig{
		GuildID:               "guild-1",
		Round:                 &round,
		ResultsChannelID:      &channel,
		NotificationChannelID: &notif,
	}
	require.NoError(t, s.UpsertGuildConfig(ctx, cfg))

	got, err := s.GetGuildConfig(ctx, "guild-1", "channel-1")
	require.NoError(t, err)
	assert.Equal(t, "guild-1", got.GuildID)
	assert.Equal(t, "Stage 1", *got.Round)
	assert.Equal(t, "channel-1", *got.ResultsChannelID)
	assert.Equal(t, "notif-1", *got.NotificationChannelID)
}

func TestGetGuildConfig_IncludesTournamentName(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	seedGuild(t, "guild-1")
	tournamentID := seedTournament(t, "IEM Cologne 2025", "swiss")

	channel := "channel-1"
	round := "Stage 1"
	cfg := GuildConfig{
		GuildID:          "guild-1",
		TournamentID:     &tournamentID,
		Round:            &round,
		ResultsChannelID: &channel,
	}
	require.NoError(t, s.UpsertGuildConfig(ctx, cfg))

	got, err := s.GetGuildConfig(ctx, "guild-1", "channel-1")
	require.NoError(t, err)
	require.NotNil(t, got.TournamentName)
	require.NotNil(t, got.Format)
	assert.Equal(t, "IEM Cologne 2025", *got.TournamentName)
	assert.Equal(t, "swiss", *got.Format)
}

func TestGetGuildConfig_NotFound(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	_, err := s.GetGuildConfig(ctx, "nonexistent-guild", "channel-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetGuildConfig")
}

func TestEnsureGuild_InsertsRow(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	require.NoError(t, s.EnsureGuild(ctx, "guild-1"))

	var count int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT count(*) FROM guilds WHERE guild_id = $1`, "guild-1").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestEnsureGuild_Idempotent(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	require.NoError(t, s.EnsureGuild(ctx, "guild-1"))
	require.NoError(t, s.EnsureGuild(ctx, "guild-1"))

	var count int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT count(*) FROM guilds WHERE guild_id = $1`, "guild-1").Scan(&count))
	assert.Equal(t, 1, count)
}

// TestEnsureGuild_SatisfiesGuildConfigFK is the reason this method exists: on a
// brand-new guild with no seeded row, EnsureGuild must create the parent guilds
// row so a first-time /config upsert doesn't fail the guild_config FK.
func TestEnsureGuild_SatisfiesGuildConfigFK(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	// Deliberately no seedGuild — EnsureGuild is the only thing creating the parent.
	require.NoError(t, s.EnsureGuild(ctx, "guild-1"))

	round := "Stage 1"
	channel := "channel-1"
	err := s.UpsertGuildConfig(ctx, GuildConfig{
		GuildID:          "guild-1",
		Round:            &round,
		ResultsChannelID: &channel,
	})
	require.NoError(t, err)
}
