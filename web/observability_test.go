/* observability_test.go
 * Unit tests for the /poller telemetry endpoint.
 */

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pickems-bot/app"
	"pickems-bot/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTelemetryServer(dataSource string) *TelemetryServer {
	return &TelemetryServer{
		app:        app.NewTestApp(app.NewMockStore("swiss", "test_round")),
		dataSource: dataSource,
	}
}

func TestPollerStatusHandler_Liquipedia_NotEnabled(t *testing.T) {
	s := newTestTelemetryServer("liquipedia")
	req := httptest.NewRequest(http.MethodGet, "/poller", nil)
	rec := httptest.NewRecorder()

	s.pollerStatusHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp PollerStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Enabled)
	assert.NotEmpty(t, resp.Message)
	assert.Nil(t, resp.LastPollAt)
}

func TestPollerStatusHandler_PandaScore_EmptyPool(t *testing.T) {
	s := newTestTelemetryServer("pandascore")
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

func TestPollerStatusHandler_PandaScore_ReturnsTrackedAndLastPoll(t *testing.T) {
	s := newTestTelemetryServer("pandascore")
	mockStore := s.app.Store.(*app.MockStore)
	mockStore.Tournaments = []store.Tournament{{ID: 42, Name: "IEM Cologne 2026"}}
	s.app.MonitoringPool.Entries[42] = &app.PoolEntry{
		DBTournamentID:         42,
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
	s := newTestTelemetryServer("pandascore")
	req := httptest.NewRequest(http.MethodPost, "/poller", nil)
	rec := httptest.NewRecorder()

	s.pollerStatusHandler(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
