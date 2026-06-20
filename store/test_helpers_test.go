//go:build integration

package store

import (
	"context"
	"os"
	"strings"
	"testing"

	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	testPool    *pgxpool.Pool
	testConnStr string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("pickems_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		panic("failed to start postgres container: " + err.Error())
	}
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("failed to get connection string: " + err.Error())
	}

	// Run migrations using pgx5:// scheme
	migrateConnStr := strings.Replace(connStr, "postgres://", "pgx5://", 1)
	migrateConnStr = strings.Replace(migrateConnStr, "postgresql://", "pgx5://", 1)

	mig, err := migrate.New("file://../migrations", migrateConnStr)
	if err != nil {
		panic("failed to create migrator: " + err.Error())
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		panic("failed to run migrations: " + err.Error())
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		panic("failed to create pool: " + err.Error())
	}
	defer pool.Close()

	testPool = pool
	testConnStr = connStr

	os.Exit(m.Run())
}

// newTestStore wraps testPool with nil fetcher and nil logger.
func newTestStore(t *testing.T) *PostgresStore {
	t.Helper()
	return &PostgresStore{pool: testPool, fetcher: nil, log: nil}
}

// newTestStoreWithFetcher wraps testPool with the provided fetcher.
func newTestStoreWithFetcher(t *testing.T, f DataSourceFetcher) *PostgresStore {
	t.Helper()
	return &PostgresStore{pool: testPool, fetcher: f, log: nil}
}

// seedTournamentNullFormat inserts a tournament with NULL format and returns its id.
func seedTournamentNullFormat(t *testing.T, name string) int {
	t.Helper()
	var id int
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO tournaments (external_id, source, name) VALUES ($1, 'test', $2) RETURNING id`,
		name, name).Scan(&id)
	require.NoError(t, err)
	return id
}

// cleanDB truncates all tables and restarts sequences.
func cleanDB(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `
		TRUNCATE guilds, users, teams, team_rankings, tournaments, matches,
		         guild_config, predictions, swiss_picks, single_elim_picks, scores
		RESTART IDENTITY CASCADE
	`)
	require.NoError(t, err)
}

// seedGuild inserts a guild row.
func seedGuild(t *testing.T, guildID string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO guilds (guild_id) VALUES ($1) ON CONFLICT DO NOTHING`, guildID)
	require.NoError(t, err)
}

// seedUser inserts a user row.
func seedUser(t *testing.T, userID, username string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO users (user_id, username) VALUES ($1, $2) ON CONFLICT (user_id) DO UPDATE SET username = EXCLUDED.username`,
		userID, username)
	require.NoError(t, err)
}

// seedTeam inserts a team row and returns its id.
func seedTeam(t *testing.T, canonicalName, source, externalID string) int {
	t.Helper()
	var id int
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO teams (canonical_name, source, external_id) VALUES ($1, $2, $3) RETURNING id`,
		canonicalName, source, externalID).Scan(&id)
	require.NoError(t, err)
	return id
}

// seedTournament inserts a tournament row with source="test" and returns its id.
func seedTournament(t *testing.T, name, format string) int {
	t.Helper()
	var id int
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO tournaments (external_id, source, name, format) VALUES ($1, 'test', $2, $3) RETURNING id`,
		name, name, format).Scan(&id)
	require.NoError(t, err)
	return id
}

// seedMatch inserts a match with team names (no FK) and returns its id.
func seedMatch(t *testing.T, tournamentID int, round, team1Name, team2Name, status string) int {
	t.Helper()
	var id int
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO matches (tournament_id, round, team1_name, team2_name, status) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		tournamentID, round, team1Name, team2Name, status).Scan(&id)
	require.NoError(t, err)
	return id
}
