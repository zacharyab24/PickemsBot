//go:build integration

package web

import (
	"context"
	"log/slog"
	"testing"

	"pickems-bot/app"
	"pickems-bot/config"
	"pickems-bot/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadPollerTestCfg loads the integration config with PandaScore as the source.
func loadPollerTestCfg(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load("../testdata/integration_config.toml")
	require.NoError(t, err)
	return cfg
}

// newPollerTestApp creates a real App for poller integration tests.
// The rate limiter is set to allow unlimited calls so tick() is never gated.
func newPollerTestApp(t *testing.T, cfg config.Config, mongoURI string) *app.App {
	t.Helper()
	t.Setenv("PANDASCORE_API_KEY", "pickems-test-key")

	a, err := app.NewApp(cfg, mongoURI, slog.Default())
	require.NoError(t, err)

	// Override to unlimited so rate limiting doesn't interfere with test ticks.
	app.SetRateLimiterForTesting(a, newUnlimitedLimiter())

	t.Cleanup(func() {
		_ = a.Store.GetClient().Disconnect(context.Background())
	})
	return a
}

// newTestPoller creates a Poller wired to the integration test server.
func newTestPoller(t *testing.T, a *app.App, cfg config.Config) *Poller {
	t.Helper()
	return NewPoller(a, cfg.PandaScore.SeriesID, "pickems-test-key", cfg.PandaScore.APIURL, nil)
}

// TestPoller_NoOp verifies that when the server state is unchanged between
// ticks, the poller does not call UpdateMatchResults and continues running.
func TestPoller_NoOp(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	testhelpers.SetState(t, "pandascore", "ongoing")

	cfg := loadPollerTestCfg(t)
	a := newPollerTestApp(t, cfg, mongoURI)

	// Populate so the App's store has data and rate limiter is primed.
	require.NoError(t, a.PopulateMatches(false))

	p := newTestPoller(t, a, cfg)

	// First tick: establishes the knownStatus baseline; no previous state to
	// compare against so no transition is detected.
	assert.True(t, p.tick(), "first tick should return true (keep going)")
	assert.NotEmpty(t, p.knownStatus, "knownStatus should be populated after first tick")
	baselineStatus := make(map[string]string, len(p.knownStatus))
	for k, v := range p.knownStatus {
		baselineStatus[k] = v
	}

	// Second tick: same state — no match transitioned to finished, so
	// UpdateMatchResults must NOT be called and the poller must keep going.
	assert.True(t, p.tick(), "second tick (no change) should return true")
	assert.Equal(t, baselineStatus, p.knownStatus,
		"knownStatus should be unchanged when server state has not changed")
}

// TestPoller_Transition verifies that when a match transitions to finished
// between ticks, the poller calls UpdateMatchResults and keeps running.
func TestPoller_Transition(t *testing.T) {
	mongoURI := testhelpers.MongoURI(t)
	testhelpers.SetState(t, "pandascore", "ongoing")

	cfg := loadPollerTestCfg(t)
	a := newPollerTestApp(t, cfg, mongoURI)
	require.NoError(t, a.PopulateMatches(false))

	p := newTestPoller(t, a, cfg)

	// First tick: establish baseline. State is "ongoing".
	assert.True(t, p.tick(), "first tick should return true")
	assert.NotEmpty(t, p.knownStatus)

	// Advance to "complete": one additional match moves from not_started/running
	// to finished. The poller must detect this transition and call UpdateMatchResults.
	testhelpers.SetState(t, "pandascore", "complete")

	// Second tick: detects the transition and updates match results.
	assert.True(t, p.tick(), "second tick (with transition) should return true")

	// Verify at least one match is now "finished" in knownStatus.
	finishedCount := 0
	for _, status := range p.knownStatus {
		if status == "finished" {
			finishedCount++
		}
	}
	assert.Greater(t, finishedCount, 0,
		"after transition tick, at least one match should be finished in knownStatus")
}
