//go:build integration

package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"pickems-bot/config"
	"pickems-bot/sources"
	"pickems-bot/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// loadIntegrationConfig loads the shared integration test config.
func loadIntegrationConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load("../testdata/integration_config.toml")
	require.NoError(t, err, "failed to load integration_config.toml")
	return cfg
}

// newTestApp creates an App connected to the test MongoDB using the provided config.
// The rate limiter is set to allow calls immediately so tests are not gated.
// t.Cleanup disconnects the client and drops the test database.
func newTestApp(t *testing.T, cfg config.Config, mongoURI string) *App {
	t.Helper()
	a, err := NewApp(cfg, mongoURI, slog.Default())
	require.NoError(t, err, "NewApp failed")

	// Override the rate limiter to allow unlimited calls so rapid sequential
	// calls within a single test are never gated.
	a.rateLimiter = rate.NewLimiter(rate.Inf, 1)

	t.Cleanup(func() {
		_ = a.Store.GetClient().Disconnect(context.Background())
	})
	return a
}

// ───────────────────────────── PandaScore ─────────────────────────────

// TestApp_PandaScore_InitialFetch verifies that PopulateMatches with the
// test server in "ongoing" state stores a non-empty set of match nodes.
func TestApp_PandaScore_InitialFetch(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	t.Setenv("PANDASCORE_API_KEY", "pickems-test-key")

	testhelpers.SetState(t, "pandascore", "ongoing")

	cfg := loadIntegrationConfig(t)
	a := newTestApp(t, cfg, mongoURI)

	require.NoError(t, a.PopulateMatches(false))

	nodes, _, err := a.Store.FetchMatchNodesFromDb()
	require.NoError(t, err)
	assert.NotEmpty(t, nodes, "expected match nodes to be stored after PopulateMatches")
}

// TestApp_PandaScore_Empty verifies that PopulateMatches with the test server
// in "not_started" state either returns an error or leaves the store empty
// without panicking.
func TestApp_PandaScore_Empty(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	t.Setenv("PANDASCORE_API_KEY", "pickems-test-key")

	testhelpers.SetState(t, "pandascore", "not_started")

	cfg := loadIntegrationConfig(t)
	a := newTestApp(t, cfg, mongoURI)

	err := a.PopulateMatches(false)
	// Either an error is returned, or the store is empty — the bot must not crash.
	if err == nil {
		nodes, _, dbErr := a.Store.FetchMatchNodesFromDb()
		require.NoError(t, dbErr)
		assert.Empty(t, nodes, "expected no match nodes stored for empty response")
	}
	// err != nil is also acceptable; just ensure no panic occurred.
}

// TestApp_PandaScore_NotFound verifies that an unknown series ID causes
// PopulateMatches to return an ErrUnrecoverable error.
func TestApp_PandaScore_NotFound(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	t.Setenv("PANDASCORE_API_KEY", "pickems-test-key")

	// Use a series_id that the test server doesn't recognise → 404.
	cfg := loadIntegrationConfig(t)
	cfg.PandaScore.SeriesID = 12345 // anything other than 99001 triggers 404
	a := newTestApp(t, cfg, mongoURI)

	err := a.PopulateMatches(false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, sources.ErrUnrecoverable),
		"expected ErrUnrecoverable for unknown series ID, got: %v", err)
}

// TestApp_PandaScore_WrongKey verifies that a bad API key causes PopulateMatches
// to return an ErrUnrecoverable error.
func TestApp_PandaScore_WrongKey(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	t.Setenv("PANDASCORE_API_KEY", "wrong-key")

	testhelpers.SetState(t, "pandascore", "ongoing")

	cfg := loadIntegrationConfig(t)
	a := newTestApp(t, cfg, mongoURI)

	err := a.PopulateMatches(false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, sources.ErrUnrecoverable),
		"expected ErrUnrecoverable for bad API key, got: %v", err)
}

// ───────────────────────────── Liquipedia ─────────────────────────────

// TestApp_Liquipedia_InitialFetch verifies that PopulateMatches with the test
// server in "ongoing" state stores match nodes using the Liquipedia source.
func TestApp_Liquipedia_InitialFetch(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	t.Setenv("LIQUIDPEDIADB_API_KEY", "pickems-test-key")

	testhelpers.SetState(t, "liquipedia", "ongoing")

	cfg := loadIntegrationConfig(t)
	cfg.DataSource = "liquipedia"
	a := newTestApp(t, cfg, mongoURI)

	require.NoError(t, a.PopulateMatches(false))

	nodes, _, err := a.Store.FetchMatchNodesFromDb()
	require.NoError(t, err)
	assert.NotEmpty(t, nodes, "expected match nodes to be stored after Liquipedia fetch")
}

// TestApp_Liquipedia_Empty verifies that PopulateMatches with the test server
// in "not_started" state either returns an error or leaves the store empty.
func TestApp_Liquipedia_Empty(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	t.Setenv("LIQUIDPEDIADB_API_KEY", "pickems-test-key")

	testhelpers.SetState(t, "liquipedia", "not_started")

	cfg := loadIntegrationConfig(t)
	cfg.DataSource = "liquipedia"
	a := newTestApp(t, cfg, mongoURI)

	err := a.PopulateMatches(false)
	if err == nil {
		nodes, _, dbErr := a.Store.FetchMatchNodesFromDb()
		require.NoError(t, dbErr)
		assert.Empty(t, nodes, "expected no match nodes stored for empty Liquipedia response")
	}
}

// TestApp_Liquipedia_WrongKey verifies that a bad Liquipedia API key causes an error.
func TestApp_Liquipedia_WrongKey(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	t.Setenv("LIQUIDPEDIADB_API_KEY", "wrong-key")

	testhelpers.SetState(t, "liquipedia", "ongoing")

	cfg := loadIntegrationConfig(t)
	cfg.DataSource = "liquipedia"
	a := newTestApp(t, cfg, mongoURI)

	err := a.PopulateMatches(false)
	require.Error(t, err, "expected error for bad Liquipedia API key")
}
