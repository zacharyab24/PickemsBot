/* tournaments_test.go
 * Unit tests for the TournamentSync helpers that don't require a live PandaScore fetch.
 * Authors: Zachary Bower
 */

package ingest

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"pickems-bot/app"
	"pickems-bot/internal/testutil"
	"pickems-bot/store"

	"github.com/stretchr/testify/assert"
)

func newTestSync(a *app.App) *TournamentSync {
	return &TournamentSync{app: a}
}

// finalizeFinishedTournament must fetch one last result before the caller
// unsubscribes the tournament - see runOnce's use of it for bug #11 (a
// tournament's last match can finish right before the hourly catalog sync
// marks it done, and once unsubscribed nothing else ever fetches that
// match's result again).
func TestFinalizeFinishedTournament_FetchesResultsForOwnRound(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 7, Source: "pandascore", ExternalID: "7", Round: "Grand Final"},
	}
	sync := newTestSync(app.NewTestApp(mockStore))

	if err := sync.finalizeFinishedTournament(context.Background(), 7); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if mockStore.FetchAndSaveMatchResultsCallCount != 1 {
		t.Errorf("expected exactly 1 results fetch, got %d", mockStore.FetchAndSaveMatchResultsCallCount)
	}
}

func TestFinalizeFinishedTournament_GetTournamentError_Propagates(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.GetTournamentError = fmt.Errorf("db down")
	sync := newTestSync(app.NewTestApp(mockStore))

	if err := sync.finalizeFinishedTournament(context.Background(), 7); err == nil {
		t.Error("expected the GetTournament error to propagate")
	}
	if mockStore.FetchAndSaveMatchResultsCallCount != 0 {
		t.Error("expected no results fetch when the tournament lookup itself fails")
	}
}

func TestFinalizeFinishedTournament_ResultsFetchError_Propagates(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 7, Source: "pandascore", ExternalID: "7", Round: "Grand Final"},
	}
	mockStore.FetchAndSaveMatchResultsError = fmt.Errorf("fetch failed")
	sync := newTestSync(app.NewTestApp(mockStore))

	if err := sync.finalizeFinishedTournament(context.Background(), 7); err == nil {
		t.Error("expected the results fetch error to propagate")
	}
}

// pandaScoreCatalog serves the upcoming and running tournament listings.
func pandaScoreCatalog(upcoming, running string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/csgo/tournaments/upcoming":
			_, _ = w.Write([]byte(upcoming))
		case "/csgo/tournaments/running":
			_, _ = w.Write([]byte(running))
		default:
			http.NotFound(w, r)
		}
	}
}

func TestTournamentSync_RunOnce_SyncsUpcomingAndRunning(t *testing.T) {
	testutil.RouteDefaultClient(t, pandaScoreCatalog(
		`[{"id": 11, "name": "Playoffs", "serie_id": 3, "league": {"name": "IEM"}, "serie": {"name": "Cologne 2026"}}]`,
		`[{"id": 12, "name": "Stage 1", "serie_id": 4, "league": {"name": "BLAST"}, "serie": {"full_name": "Bounty Summer"}}]`,
	))
	ms := app.NewMockStore("swiss", "test_round")

	newTestSync(app.NewTestApp(ms)).runOnce(context.Background())

	assert.Equal(t, []store.TournamentCatalogEntry{
		{ExternalID: "11", Name: "IEM Cologne 2026", Round: "Playoffs", SeriesID: "3"},
		{ExternalID: "12", Name: "BLAST Bounty Summer", Round: "Stage 1", SeriesID: "4"},
	}, ms.SyncedTournaments)
}

// A tournament leaving the active set gets one last results fetch and is
// dropped from the pool; anything still configured but missing is re-added.
func TestTournamentSync_RunOnce_FinalizesFinishedAndReconcilesPool(t *testing.T) {
	testutil.RouteDefaultClient(t, pandaScoreCatalog(`[{"id": 11, "name": "Playoffs", "league": {"name": "IEM"}}]`, `[]`))
	ms := app.NewMockStore("swiss", "test_round")
	ms.Tournaments = []store.Tournament{
		{ID: 1, Source: "pandascore", ExternalID: "10", Round: "Group Stage"},
		{ID: 2, Source: "pandascore", ExternalID: "11", Round: "Playoffs"},
	}
	ms.SyncTournamentsNewlyFinished = []int{1}
	ms.ListTrackedTournamentIDsResult = []int{2}
	a := app.NewTestApp(ms)
	a.Subscribe(context.Background(), 1)

	newTestSync(a).runOnce(context.Background())

	assert.Equal(t, 1, ms.FetchAndSaveMatchResultsCallCount, "expected a final results fetch for the finished tournament")
	ids := map[int]bool{}
	for _, e := range a.Snapshot() {
		ids[e.DBTournamentID] = true
	}
	assert.Equal(t, map[int]bool{2: true}, ids)
}

func TestTournamentSync_RunOnce_SkipsSyncOnBadFetch(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"running fetch fails": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/csgo/tournaments/running" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`[{"id": 11, "name": "Playoffs"}]`))
		},
		"unauthorized":       func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
		"malformed response": func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{")) },
		// An empty set would mark every catalog row finished.
		"empty active set": pandaScoreCatalog(`[]`, `[]`),
	}
	for name, h := range tests {
		t.Run(name, func(t *testing.T) {
			testutil.RouteDefaultClient(t, h)
			ms := app.NewMockStore("swiss", "test_round")

			newTestSync(app.NewTestApp(ms)).runOnce(context.Background())

			assert.Nil(t, ms.SyncedTournaments)
		})
	}
}
