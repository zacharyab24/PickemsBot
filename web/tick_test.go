/* tick_test.go
 * Unit tests for Poller.tick() using httptest servers and a mock App.
 */

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pickems-bot/app"
	"pickems-bot/sources"

	"github.com/stretchr/testify/assert"
)

// notStartedMatchJSON is a minimal valid PandaScore response with no finished matches.
const notStartedMatchJSON = `[{"id": 1, "name": "Round 1", "status": "not_started", "opponents": [], "winner": null, "results": []}]`

// finishedMatchJSON is a minimal PandaScore response where match 1 is finished.
const finishedMatchJSON = `[{"id": 1, "name": "Round 1", "status": "finished",
  "opponents": [
    {"opponent": {"name": "Alpha", "id": 1}},
    {"opponent": {"name": "Beta",  "id": 2}}
  ],
  "winner": {"name": "Alpha"},
  "results": [{"team_id": 1, "score": 2}, {"team_id": 2, "score": 0}]
}]`

// newMockPoller creates a Poller backed by a mock App for unit tests.
// Named differently from the integration test helper to avoid redeclaration
// when the binary is compiled with -tags integration.
func newMockPoller(a *app.App, apiURL string) *Poller {
	return NewPoller(a, 99001, "test-key", apiURL, nil)
}

// region tick() tests

func TestTick_RateLimitReached_ReturnsTrue(t *testing.T) {
	// An App with a nil rateLimiter causes Allow() to return false →
	// tick() logs a warning and returns true (poller continues).
	mockStore := app.NewMockStore("swiss", "test_round")
	// Construct an App with nil rate limiter directly using the exported mock helper
	// NewTestApp gives unlimited; for a nil limiter we need a nil-limiter App.
	// Use a Poller whose app.Allow() returns false.
	a := &app.App{Store: mockStore} // rateLimiter is nil → Allow() returns false
	p := newMockPoller(a, "http://localhost")

	result := p.tick()
	assert.True(t, result, "tick() must return true when rate-limited (poller keeps running)")
}

func TestTick_UnrecoverableError_ReturnsFalse(t *testing.T) {
	// PandaScore returns 401 → ErrUnrecoverable → tick() returns false.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	mockStore := app.NewMockStore("swiss", "test_round")
	a := app.NewTestApp(mockStore)
	p := newMockPoller(a, srv.URL)

	result := p.tick()
	assert.False(t, result, "tick() must return false on unrecoverable error")
}

func TestTick_RetriableError_ReturnsTrue(t *testing.T) {
	// PandaScore returns 500 → retriable error → tick() returns true.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	mockStore := app.NewMockStore("swiss", "test_round")
	a := app.NewTestApp(mockStore)
	p := newMockPoller(a, srv.URL)

	result := p.tick()
	assert.True(t, result, "tick() must return true on retriable error")
}

func TestTick_ParseError_ReturnsTrue(t *testing.T) {
	// PandaScore returns invalid JSON → parse error → tick() returns true.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json {{"))
	}))
	defer srv.Close()

	mockStore := app.NewMockStore("swiss", "test_round")
	a := app.NewTestApp(mockStore)
	p := newMockPoller(a, srv.URL)

	result := p.tick()
	assert.True(t, result, "tick() must return true on parse error (will retry)")
}

func TestTick_NoTransition_ReturnsTrue(t *testing.T) {
	// Server returns not_started match; knownStatus empty → no transition → returns true.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(notStartedMatchJSON))
	}))
	defer srv.Close()

	mockStore := app.NewMockStore("swiss", "test_round")
	a := app.NewTestApp(mockStore)
	p := newMockPoller(a, srv.URL)

	result := p.tick()
	assert.True(t, result)
	assert.Equal(t, "not_started", p.knownStatus["1"])
}

func TestTick_FinishedTransition_TriggersUpdateAndReturnsTrue(t *testing.T) {
	// Simulate a match transitioning from running → finished across two ticks.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(finishedMatchJSON))
	}))
	defer srv.Close()

	mockStore := app.NewMockStore("swiss", "test_round")
	// Give the mock store some scheduled matches so EnsureScheduledMatches passes
	// (called inside GenerateLeaderboard via tick's finishedTransition branch).
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Alpha", Team2: "Beta"},
	})

	a := app.NewTestApp(mockStore)
	p := newMockPoller(a, srv.URL)

	// Seed knownStatus so the match looks like it was "running" before this tick.
	p.knownStatus["1"] = "running"

	result := p.tick()
	assert.True(t, result, "tick() must return true after processing a finished transition")
	assert.Equal(t, "finished", p.knownStatus["1"])
}

// endregion
