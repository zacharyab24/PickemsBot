/* tournaments_test.go
 * Unit tests for the TournamentSync helpers that don't require a live PandaScore fetch.
 * Authors: Zachary Bower
 */

package ingest

import (
	"context"
	"fmt"
	"testing"

	"pickems-bot/app"
	"pickems-bot/store"
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
