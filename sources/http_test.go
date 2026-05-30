/* http_test.go
 * Unit tests for the HTTP client functions in the sources package.
 * Uses httptest.Server to avoid real network calls.
 */

package sources

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// region GetPandaScoreMatches tests

func TestGetPandaScoreMatches_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		// Verify query params were set
		assert.Equal(t, "99001", r.URL.Query().Get("filter[serie_id]"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id": 1, "name": "Round 1", "status": "not_started", "opponents": [], "winner": null, "results": []}]`))
	}))
	defer srv.Close()

	body, err := GetPandaScoreMatches(srv.URL, "test-key", 99001, 0)
	require.NoError(t, err)
	assert.Contains(t, body, "not_started")
}

func TestGetPandaScoreMatches_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := GetPandaScoreMatches(srv.URL, "bad-key", 99001, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnrecoverable)
}

func TestGetPandaScoreMatches_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := GetPandaScoreMatches(srv.URL, "key", 99001, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnrecoverable)
}

func TestGetPandaScoreMatches_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := GetPandaScoreMatches(srv.URL, "key", 99001, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnrecoverable)
}

func TestGetPandaScoreMatches_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := GetPandaScoreMatches(srv.URL, "key", 99001, 0)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnrecoverable) // 500 is retriable, not unrecoverable
	assert.Contains(t, err.Error(), "unexpected status code")
}

func TestGetPandaScoreMatches_InvalidURL(t *testing.T) {
	_, err := GetPandaScoreMatches("://invalid-url", "key", 1, 0)
	require.Error(t, err)
}

// endregion

// region GetLiquipediaMatchDataByPage tests

func TestGetLiquipediaMatchDataByPage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Authorization"), "Apikey test-key")
		// Verify query params
		assert.Equal(t, "counterstrike", r.URL.Query().Get("wiki"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result": []}`))
	}))
	defer srv.Close()

	body, err := GetLiquipediaMatchDataByPage(srv.URL, "test-key", "Test/Page")
	require.NoError(t, err)
	assert.Contains(t, body, "result")
}

func TestGetLiquipediaMatchDataByPage_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := GetLiquipediaMatchDataByPage(srv.URL, "key", "Test/Page")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestGetLiquipediaMatchDataByPage_InvalidURL(t *testing.T) {
	_, err := GetLiquipediaMatchDataByPage("://bad-url", "key", "page")
	require.Error(t, err)
}

// endregion

// region GetLiquipediaMatchData tests

func TestGetLiquipediaMatchData_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Authorization"), "Apikey test-key")
		// Verify the conditions query param contains bracket IDs
		cond := r.URL.Query().Get("conditions")
		assert.Contains(t, cond, "ID1")
		assert.Contains(t, cond, "ID2")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result": []}`))
	}))
	defer srv.Close()

	body, err := GetLiquipediaMatchData(srv.URL, "test-key", []string{"ID1", "ID2"})
	require.NoError(t, err)
	assert.Contains(t, body, "result")
}

func TestGetLiquipediaMatchData_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := GetLiquipediaMatchData(srv.URL, "key", []string{"ID1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code")
}

func TestGetLiquipediaMatchData_InvalidURL(t *testing.T) {
	_, err := GetLiquipediaMatchData("://bad-url", "key", []string{"ID1"})
	require.Error(t, err)
}

// endregion
