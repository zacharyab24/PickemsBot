/* external_test.go
 * Contains unit tests for external.go HTTP functions using httptest
 * AI-Generated
 */

package external

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetWikitext_Success tests successful wikitext fetching
func TestGetWikitext_Success(t *testing.T) {
	expectedContent := "{{MatchList|id=abc123}}"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "LiquipediaDataFetcher/1.0", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedContent))
	}))
	defer server.Close()

	result, err := GetWikitext(server.URL)

	require.NoError(t, err)
	assert.Equal(t, expectedContent, result)
}

// TestGetWikitext_GzipResponse tests handling of gzip-encoded responses
func TestGetWikitext_GzipResponse(t *testing.T) {
	expectedContent := "{{MatchList|id=gzip123}}"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that client accepts gzip
		assert.Equal(t, "gzip", r.Header.Get("Accept-Encoding"))

		// Compress response
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		gzWriter.Write([]byte(expectedContent))
		gzWriter.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	result, err := GetWikitext(server.URL)

	require.NoError(t, err)
	assert.Equal(t, expectedContent, result)
}

// TestGetWikitext_ServerError tests handling of non-200 status codes
func TestGetWikitext_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	result, _ := GetWikitext(server.URL)

	assert.Empty(t, result)
	// The function returns empty string on non-200
	assert.Equal(t, "", result)
}

// TestGetWikitext_NotFound tests handling of 404 status
func TestGetWikitext_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result, _ := GetWikitext(server.URL)

	assert.Empty(t, result)
}

// TestGetWikitext_EmptyResponse tests handling of empty response body
func TestGetWikitext_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Don't write any body
	}))
	defer server.Close()

	result, err := GetWikitext(server.URL)

	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestGetWikitext_LargeResponse tests handling of large response bodies
func TestGetWikitext_LargeResponse(t *testing.T) {
	// Create a large response (10KB of data)
	largeContent := make([]byte, 10*1024)
	for i := range largeContent {
		largeContent[i] = 'a'
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(largeContent)
	}))
	defer server.Close()

	result, err := GetWikitext(server.URL)

	require.NoError(t, err)
	assert.Len(t, result, 10*1024)
}

// TestGetWikitext_LargeGzipResponse tests handling of large gzip-compressed responses
func TestGetWikitext_LargeGzipResponse(t *testing.T) {
	// Create content
	content := "{{MatchList|id=test123}}{{MatchList|id=test456}}"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		gzWriter.Write([]byte(content))
		gzWriter.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	result, err := GetWikitext(server.URL)

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

// TestGetWikitext_SpecialCharacters tests handling of special characters in response
func TestGetWikitext_SpecialCharacters(t *testing.T) {
	content := "{{Match|team1=Natus Vincere|team2=G2 Esports|score=2-1}}"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	result, err := GetWikitext(server.URL)

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

// TestGetWikitext_UTF8Content tests handling of UTF-8 encoded content
func TestGetWikitext_UTF8Content(t *testing.T) {
	content := "{{Match|team1=Мongolz|team2=チーム}}"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	result, err := GetWikitext(server.URL)

	require.NoError(t, err)
	assert.Equal(t, content, result)
}
