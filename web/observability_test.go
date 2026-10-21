/* observability_test.go
 * Unit tests for the /poller telemetry endpoint.
 */

package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pickems-bot/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTelemetryServer() *TelemetryServer {
	return &TelemetryServer{
		app: app.NewTestApp(app.NewMockStore("swiss", "test_round")),
	}
}

func TestPollerStatusHandler_EmptyPool(t *testing.T) {
	s := newTestTelemetryServer()
	req := httptest.NewRequest(http.MethodGet, "/poller", nil)
	rec := httptest.NewRecorder()

	s.pollerStatusHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp PollerStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Enabled)
	assert.Empty(t, resp.Tracked)
	assert.Nil(t, resp.LastPollAt)
}

func TestPollerStatusHandler_ReturnsTrackedAndLastPoll(t *testing.T) {
	s := newTestTelemetryServer()
	// Name is read straight off the pool entry - see PoolEntry.Name - so no
	// store lookup (and no N+1 across tracked tournaments) is needed here.
	s.app.MonitoringPool.Entries[42] = &app.PoolEntry{
		DBTournamentID:         42,
		Name:                   "IEM Cologne 2026",
		PandascoreTournamentID: 555,
		SeriesID:               999,
		Round:                  "Playoffs",
	}
	s.app.RecordPoll()

	req := httptest.NewRequest(http.MethodGet, "/poller", nil)
	rec := httptest.NewRecorder()
	s.pollerStatusHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp PollerStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Enabled)
	require.Len(t, resp.Tracked, 1)
	assert.Equal(t, 42, resp.Tracked[0].TournamentID)
	assert.Equal(t, "IEM Cologne 2026", resp.Tracked[0].Name)
	assert.Equal(t, 555, resp.Tracked[0].PandascoreTournamentID)
	assert.Equal(t, 999, resp.Tracked[0].SeriesID)
	assert.Equal(t, "Playoffs", resp.Tracked[0].Round)
	assert.NotNil(t, resp.LastPollAt)
}

func TestPollerStatusHandler_MethodNotAllowed(t *testing.T) {
	s := newTestTelemetryServer()
	req := httptest.NewRequest(http.MethodPost, "/poller", nil)
	rec := httptest.NewRecorder()

	s.pollerStatusHandler(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

type fakeDiscord bool

func (f fakeDiscord) IsConnected() bool { return bool(f) }

func TestHealthHandler(t *testing.T) {
	tests := map[string]struct {
		pingErr   error
		connected bool
		wantCode  int
		want      ResponseChecks
	}{
		"all healthy":          {nil, true, http.StatusOK, ResponseChecks{Postgres: "ok", Discord: "ok"}},
		"postgres down":        {errors.New("refused"), true, http.StatusServiceUnavailable, ResponseChecks{Postgres: "error: refused", Discord: "ok"}},
		"discord disconnected": {nil, false, http.StatusServiceUnavailable, ResponseChecks{Postgres: "ok", Discord: "disconnected"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ms := app.NewMockStore("swiss", "test_round")
			ms.PingError = tt.pingErr
			s := &TelemetryServer{app: app.NewTestApp(ms), discord: fakeDiscord(tt.connected), startTime: time.Now()}
			rec := httptest.NewRecorder()

			s.healthHandler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

			require.Equal(t, tt.wantCode, rec.Code)
			var resp HealthResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.Equal(t, tt.want, resp.Checks)
		})
	}
}
