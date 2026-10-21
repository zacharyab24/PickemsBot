package sources

import (
	"errors"
	"net/http"
	"testing"

	"pickems-bot/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPandaScoreTournament_DisplayName(t *testing.T) {
	tests := map[string]struct {
		league, serie, fullName string
		want                    string
	}{
		"serie name preferred":      {"IEM", "Cologne 2026", "IEM Cologne Major 2026", "IEM Cologne 2026"},
		"falls back to full name":   {"BLAST", "", "Bounty Summer 2026", "BLAST Bounty Summer 2026"},
		"falls back to league only": {"PGL", "", "", "PGL"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var pt PandaScoreTournament
			pt.League.Name, pt.Serie.Name, pt.Serie.FullName = tt.league, tt.serie, tt.fullName
			assert.Equal(t, tt.want, pt.DisplayName())
		})
	}
}

func TestCountTeams_DistinctAndSkipsTBD(t *testing.T) {
	team := func(id int) BracketOpponent { return BracketOpponent{Opponent: &BracketTeam{ID: id}} }
	matches := []BracketMatch{
		{Opponents: []BracketOpponent{team(1), team(2)}},
		{Opponents: []BracketOpponent{team(3), team(1)}},
		{Opponents: []BracketOpponent{{Opponent: nil}, team(0)}},
	}

	assert.Equal(t, 3, CountTeams(matches))
}

func TestGetPandaScoreTournaments_DecodesListing(t *testing.T) {
	var gotAuth, gotPath string
	testutil.RouteDefaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("authorization"), r.URL.Path
		_, _ = w.Write([]byte(`[{"id": 7, "name": "Playoffs", "serie_id": 3, "league": {"name": "IEM"}, "serie": {"name": "Cologne"}}]`))
	})

	got, err := GetRunningPandaScoreTournaments("secret")
	require.NoError(t, err)

	assert.Equal(t, "Bearer secret", gotAuth)
	assert.Equal(t, "/csgo/tournaments/running", gotPath)
	require.Len(t, got, 1)
	assert.Equal(t, 7, got[0].ID)
	assert.Equal(t, "Playoffs", got[0].Round)
	assert.Equal(t, 3, got[0].SerieID)
	assert.Equal(t, "IEM Cologne", got[0].DisplayName())
}

func TestGetPandaScoreBracket_DecodesEdges(t *testing.T) {
	var gotPath string
	testutil.RouteDefaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[{"id": 2, "previous_matches": [{"match_id": 1, "type": "loser"}], "opponents": [{"opponent": {"id": 5, "name": "A"}}, {"opponent": null}]}]`))
	})

	got, err := GetPandaScoreBracket("key", 42)
	require.NoError(t, err)

	assert.Equal(t, "/tournaments/42/brackets", gotPath)
	require.Len(t, got, 1)
	assert.Equal(t, []BracketEdge{{FromMatchID: 1, Type: "loser"}}, got[0].PreviousMatches)
	assert.Nil(t, got[0].Opponents[1].Opponent)
}

// Both catalog fetchers and the bracket fetcher share one status mapping.
func TestPandaScoreCatalogFetchers_StatusHandling(t *testing.T) {
	fetchers := map[string]func() error{
		"upcoming": func() error { _, err := GetUpcomingPandaScoreTournaments("k"); return err },
		"running":  func() error { _, err := GetRunningPandaScoreTournaments("k"); return err },
		"bracket":  func() error { _, err := GetPandaScoreBracket("k", 1); return err },
	}
	tests := map[string]struct {
		status            int
		body              string
		wantUnrecoverable bool
	}{
		"unauthorized":   {http.StatusUnauthorized, "", true},
		"forbidden":      {http.StatusForbidden, "", true},
		"not found":      {http.StatusNotFound, "", true},
		"server error":   {http.StatusBadGateway, "", false},
		"malformed body": {http.StatusOK, "{", false},
	}
	for fetcher, fetch := range fetchers {
		for name, tt := range tests {
			t.Run(fetcher+"/"+name, func(t *testing.T) {
				testutil.RouteDefaultClient(t, func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(tt.body))
				})

				err := fetch()

				require.Error(t, err)
				assert.Equal(t, tt.wantUnrecoverable, errors.Is(err, ErrUnrecoverable))
			})
		}
	}
}
