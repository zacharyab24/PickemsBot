/* observability_test.go
 * Contains unit tests for observability.go (TelemetryServer health handler)
 * Authors: Zachary Bower
 */

package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiPkg "pickems-bot/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDiscord satisfies the interface{ IsConnected() bool } used by TelemetryServer
type mockDiscord struct {
	connected bool
}

func (m *mockDiscord) IsConnected() bool { return m.connected }

// newTestTelemetryServer builds a TelemetryServer wired to a MockStore and mockDiscord.
func newTestTelemetryServer(pingErr error, discordConnected bool) *TelemetryServer {
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.PingError = pingErr
	return &TelemetryServer{
		app:       apiPkg.NewTestApp(mockStore),
		discord:   &mockDiscord{connected: discordConnected},
		startTime: time.Now(),
	}
}

// region healthHandler tests

func TestHealthHandler_Healthy(t *testing.T) {
	s := newTestTelemetryServer(nil, true)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp HealthResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "ok", resp.Checks.MongoDb)
	assert.Equal(t, "ok", resp.Checks.Discord)
}

func TestHealthHandler_MongoDown(t *testing.T) {
	s := newTestTelemetryServer(errors.New("connection refused"), true)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp HealthResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "unhealthy", resp.Status)
	assert.Contains(t, resp.Checks.MongoDb, "error:")
	assert.Equal(t, "ok", resp.Checks.Discord)
}

func TestHealthHandler_DiscordDisconnected(t *testing.T) {
	s := newTestTelemetryServer(nil, false)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp HealthResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "unhealthy", resp.Status)
	assert.Equal(t, "ok", resp.Checks.MongoDb)
	assert.Equal(t, "disconnected", resp.Checks.Discord)
}

func TestHealthHandler_BothDown(t *testing.T) {
	s := newTestTelemetryServer(errors.New("timeout"), false)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp HealthResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "unhealthy", resp.Status)
	assert.Contains(t, resp.Checks.MongoDb, "error:")
	assert.Equal(t, "disconnected", resp.Checks.Discord)
}

func TestHealthHandler_UptimeIsNonNegative(t *testing.T) {
	s := newTestTelemetryServer(nil, true)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, req)

	var resp HealthResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.GreaterOrEqual(t, resp.Uptime, int64(0))
}

func TestHealthHandler_HeadRequest_Healthy_Returns200(t *testing.T) {
	s := newTestTelemetryServer(nil, true)

	req := httptest.NewRequest(http.MethodHead, "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, req)

	// Body stripping for HEAD is done by the real net/http server, not the handler.
	// In unit tests with httptest.ResponseRecorder we only assert on the status code.
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthHandler_HeadRequest_Unhealthy_Returns503(t *testing.T) {
	s := newTestTelemetryServer(errors.New("mongo down"), false)

	req := httptest.NewRequest(http.MethodHead, "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHealthHandler_InvalidMethod_Returns405(t *testing.T) {
	s := newTestTelemetryServer(nil, true)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/health", nil)
		w := httptest.NewRecorder()

		s.healthHandler(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "expected 405 for method %s", method)
	}
}

// endregion
