/* datasource_test.go
 * Unit tests for LiquipediaFetcher and PandaScoreFetcher using httptest servers.
 */

package store

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pickems-bot/tournament"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Minimal Liquipedia JSON for a single Swiss-format match node.
// Section must contain "round" so DetectKindFromMatchNodes identifies Swiss format.
const liqSwissJSON = `{
  "result": [
    {
      "match2id": "TEST_R01-M001",
      "finished": 0,
      "match2opponents": [{"name": "Alpha"}, {"name": "Beta"}],
      "section": "Round 1",
      "bestof": 3
    }
  ]
}`

// Minimal Liquipedia schedule JSON (one upcoming match).
const liqScheduleJSON = `{
  "result": [
    {
      "finished": 0,
      "date": "2025-01-15 14:00:00",
      "stream": {"twitch": "eslcs"},
      "bestof": 3,
      "match2opponents": [{"name": "Alpha"}, {"name": "Beta"}]
    }
  ]
}`

// Minimal PandaScore JSON for a single Swiss-format match node.
// Name must contain "Round" so DetectKindFromMatchNodes (via Section field) identifies Swiss.
const psMatchJSON = `[
  {
    "id": 1,
    "name": "Round 1",
    "status": "not_started",
    "opponents": [
      {"opponent": {"name": "Alpha", "id": 1}},
      {"opponent": {"name": "Beta",  "id": 2}}
    ],
    "winner": null,
    "results": []
  }
]`

// Minimal PandaScore JSON shaped like a group-stage/double-elimination match -
// its "name" ("Winners' match") contains none of the keywords
// DetectKindFromMatchNodes looks for ("round", "final"/"quarterfinal"/
// "semifinal", "upper", "lower"), so section-based auto-detection can't
// classify it even though it's a perfectly real, already-known format.
const psGroupStageJSON = `[
  {
    "id": 1,
    "name": "Winners' match",
    "status": "not_started",
    "opponents": [
      {"opponent": {"name": "Alpha", "id": 1}},
      {"opponent": {"name": "Beta",  "id": 2}}
    ],
    "winner": null,
    "results": []
  }
]`

// Minimal PandaScore schedule JSON.
const psScheduleJSON = `[
  {
    "status": "not_started",
    "scheduled_at": "2025-01-15T14:00:00Z",
    "number_of_games": 3,
    "opponents": [
      {"opponent": {"name": "Alpha"}},
      {"opponent": {"name": "Beta"}}
    ],
    "streams_list": []
  }
]`

// region NewPandaScoreFetcher tests

func TestNewPandaScoreFetcher_Fields(t *testing.T) {
	f := NewPandaScoreFetcher("http://example.com", "my-key", 42, 0)
	assert.Equal(t, "http://example.com", f.apiURL)
	assert.Equal(t, "my-key", f.apiKey)
	assert.Equal(t, 42, f.seriesID)
}

// endregion

// region LiquipediaFetcher tests

func TestLiquipediaFetcher_FetchMatchData_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(liqSwissJSON))
	}))
	defer srv.Close()

	f := NewLiquipediaFetcher(srv.URL, "test-key", "Test/Page")
	result, nodes, err := f.FetchMatchData("Round 1", "")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, nodes)
}

func TestLiquipediaFetcher_FetchMatchData_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := NewLiquipediaFetcher(srv.URL, "key", "Test/Page")
	_, _, err := f.FetchMatchData("Round 1", "")
	require.Error(t, err)
}

func TestLiquipediaFetcher_FetchSchedule_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(liqScheduleJSON))
	}))
	defer srv.Close()

	f := NewLiquipediaFetcher(srv.URL, "test-key", "Test/Page")
	matches, err := f.FetchSchedule()
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "Alpha", matches[0].Team1)
	assert.Equal(t, "Beta", matches[0].Team2)
}

func TestLiquipediaFetcher_FetchSchedule_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	f := NewLiquipediaFetcher(srv.URL, "key", "Test/Page")
	_, err := f.FetchSchedule()
	require.Error(t, err)
}

// endregion

// region PandaScoreFetcher tests

func TestPandaScoreFetcher_FetchMatchData_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(psMatchJSON))
	}))
	defer srv.Close()

	f := NewPandaScoreFetcher(srv.URL, "test-key", 99001, 0)
	result, nodes, err := f.FetchMatchData("Round 1", "")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, nodes)
}

func TestPandaScoreFetcher_FetchMatchData_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	f := NewPandaScoreFetcher(srv.URL, "key", 99001, 0)
	_, _, err := f.FetchMatchData("Round 1", "")
	require.Error(t, err)
}

// TestPandaScoreFetcher_FetchMatchData_UnrecognisedSection_FailsWithoutKnownKind
// and the "_KnownKindBypassesDetection" test below are the regression pair for
// the bug where FetchMatchData re-derived a format from match node sections
// even when the tournament's real format was already known - section-based
// detection doesn't recognise every bracket shape (e.g. this group-stage
// naming), even though the already-persisted format is perfectly valid.
func TestPandaScoreFetcher_FetchMatchData_UnrecognisedSection_FailsWithoutKnownKind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(psGroupStageJSON))
	}))
	defer srv.Close()

	f := NewPandaScoreFetcher(srv.URL, "test-key", 99001, 0)
	_, _, err := f.FetchMatchData("Group B", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not detect tournament format")
}

func TestPandaScoreFetcher_FetchMatchData_KnownKindBypassesDetection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(psGroupStageJSON))
	}))
	defer srv.Close()

	f := NewPandaScoreFetcher(srv.URL, "test-key", 99001, 0)
	_, _, err := f.FetchMatchData("Group B", tournament.DoubleElim)
	// Detection is skipped entirely - this reaches the registered
	// unsupported-format placeholder instead of the section-detection error.
	require.ErrorIs(t, err, tournament.ErrPredictionsUnsupported)
}

func TestPandaScoreFetcher_FetchSchedule_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(psScheduleJSON))
	}))
	defer srv.Close()

	f := NewPandaScoreFetcher(srv.URL, "test-key", 99001, 0)
	matches, err := f.FetchSchedule()
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "Alpha", matches[0].Team1)
}

func TestPandaScoreFetcher_FetchSchedule_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	f := NewPandaScoreFetcher(srv.URL, "key", 99001, 0)
	_, err := f.FetchSchedule()
	require.Error(t, err)
}

// endregion
