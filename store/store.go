package store

import (
	"context"
	"fmt"
	"log/slog"
	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/tournament"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Interface defines the methods that the Store struct must implement. This allows for easier testing and abstraction.
type Interface interface {
	// Health
	Ping(ctx context.Context) error
	Close()

	// Tournament lifecycle — format is detected lazily from match data, not stored at creation
	EnsureTournament(ctx context.Context, externalID, source, name string, seriesID int) (int, error)

	// Guild config — new in v4, used by /config (#67)
	GetGuildConfig(ctx context.Context, guildID, channelID string) (GuildConfig, error)
	UpsertGuildConfig(ctx context.Context, cfg GuildConfig) error

	// Match data
	EnsureScheduledMatches(ctx context.Context, tournamentID int) error
	ListValidTeams(ctx context.Context, tournamentID int, round string) ([]string, tournament.Kind, error)
	GetMatchResults(ctx context.Context, tournamentID int, round string) (tournament.MatchResult, error)
	UpsertMatchResults(ctx context.Context, tournamentID int, result tournament.MatchResult) error
	FetchAndSaveMatchResults(ctx context.Context, tournamentID int, round string) error
	GetMatchNodes(ctx context.Context, tournamentID int, round string) ([]sources.MatchNode, tournament.Kind, error)

	// Schedule
	GetMatchSchedule(ctx context.Context, tournamentID int) ([]sources.ScheduledMatch, error)
	UpsertMatchSchedule(ctx context.Context, tournamentID int, matches []sources.ScheduledMatch) error
	FetchAndSaveSchedule(ctx context.Context, tournamentID int) error

	// Predictions
	UpsertPrediction(ctx context.Context, guildID string, tournamentID int, prediction models.Prediction) error
	GetPrediction(ctx context.Context, userID, guildID string, tournamentID int, round string) (models.Prediction, error)
	GetPredictionByUsername(ctx context.Context, username, guildID string, tournamentID int, round string) (models.Prediction, error)
	ListPredictions(ctx context.Context, guildID string, tournamentID int, round string) ([]models.Prediction, error)

	// Leaderboard — scores are materialised on match result insert, not stored separately
	GetLeaderboard(ctx context.Context, guildID string, tournamentID int) ([]LeaderboardEntry, error)

	// VRS
	ListVRSRankings(ctx context.Context) ([]VRSEntry, error)
}

// PostgresStore represents the database connection and configuration
type PostgresStore struct {
	pool    *pgxpool.Pool
	fetcher DataSourceFetcher
	log     *slog.Logger
}

// logger returns the store's logger, falling back to the global default when none was injected.
func (s *PostgresStore) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}

// NewStore initializes a new PostgresStore with the given connection string, data source fetcher, and logger.
func NewStore(connString string, fetcher DataSourceFetcher, log *slog.Logger) (*PostgresStore, error) {
	if connString == "" {
		return nil, fmt.Errorf("postgres connection string is empty: set POSTGRES_URI in .env")
	}
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, err
	}

	return &PostgresStore{
		pool:    pool,
		fetcher: fetcher,
		log:     log,
	}, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

var _ Interface = (*PostgresStore)(nil)
