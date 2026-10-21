/* poller_test.go
 * Contains unit tests for poller.go functions
 * Authors: Zachary Bower
 */

package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"pickems-bot/app"
	"pickems-bot/sources"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// region tick tests

// fakePandaScore serves whatever status and body the test last set.
type fakePandaScore struct {
	status int
	body   string
	query  url.Values
}

func newFakePandaScore(t *testing.T) (*fakePandaScore, *httptest.Server) {
	t.Helper()
	f := &fakePandaScore{status: http.StatusOK, body: "[]"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.query = r.URL.Query()
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

// psMatch renders one PandaScore match in tournament 55. winner is only set
// when status is "finished".
func psMatch(id int, status, scheduledAt string) string {
	winner := "null"
	if status == "finished" {
		winner = `{"name": "Alpha"}`
	}
	return fmt.Sprintf(`{
		"id": %d, "name": "Round 1", "status": %q, "tournament_id": 55,
		"scheduled_at": %q, "number_of_games": 3,
		"opponents": [{"opponent": {"name": "Alpha", "id": 1}}, {"opponent": {"name": "Beta", "id": 2}}],
		"winner": %s, "results": [{"team_id": 1, "score": 2}, {"team_id": 2, "score": 0}],
		"streams_list": []
	}`, id, status, scheduledAt, winner)
}

func newTickPoller(t *testing.T, apiURL string) (*Poller, *app.MockStore, *app.PoolEntry) {
	t.Helper()
	ms := app.NewMockStore("swiss", "Stage 1")
	p := NewPoller(app.NewTestApp(ms), "key", apiURL, nil)
	entry := &app.PoolEntry{DBTournamentID: 1, PandascoreTournamentID: 55, Round: "Stage 1", KnownStatus: map[string]string{}}
	return p, ms, entry
}

func TestTick_MatchFinishing_FetchesResults(t *testing.T) {
	fake, srv := newFakePandaScore(t)
	p, ms, entry := newTickPoller(t, srv.URL)

	fake.body = "[" + psMatch(1, "running", "2030-01-01T10:00:00Z") + "]"
	require.True(t, p.tick(entry))
	assert.Equal(t, 0, ms.FetchAndSaveMatchResultsCallCount, "no results fetch while the match is still running")

	fake.body = "[" + psMatch(1, "finished", "2030-01-01T10:00:00Z") + "]"
	require.True(t, p.tick(entry))
	assert.Equal(t, 1, ms.FetchAndSaveMatchResultsCallCount, "running -> finished should trigger a results fetch")

	require.True(t, p.tick(entry))
	assert.Equal(t, 1, ms.FetchAndSaveMatchResultsCallCount, "an already-finished match should not trigger again")
}

// On restart the pool starts empty, so a match first seen as finished is
// history, not a transition.
func TestTick_FirstSeenFinished_DoesNotFetchResults(t *testing.T) {
	fake, srv := newFakePandaScore(t)
	p, ms, entry := newTickPoller(t, srv.URL)
	fake.body = "[" + psMatch(1, "finished", "2030-01-01T10:00:00Z") + "]"

	require.True(t, p.tick(entry))

	assert.Equal(t, 0, ms.FetchAndSaveMatchResultsCallCount)
	assert.Equal(t, "finished", entry.KnownStatus["1"])
}

func TestTick_StoresScheduleOnlyWhenItChanges(t *testing.T) {
	fake, srv := newFakePandaScore(t)
	p, ms, entry := newTickPoller(t, srv.URL)
	fake.body = "[" + psMatch(1, "not_started", "2030-01-01T10:00:00Z") + "]"

	require.True(t, p.tick(entry))
	require.True(t, p.tick(entry))
	assert.Equal(t, 1, ms.StoreMatchScheduleCallCount, "an unchanged schedule should not be rewritten")
	require.Len(t, ms.ScheduledMatches, 1)
	assert.Equal(t, "Alpha", ms.ScheduledMatches[0].Team1)

	fake.body = "[" + psMatch(1, "not_started", "2030-01-01T12:00:00Z") + "]"
	require.True(t, p.tick(entry))
	assert.Equal(t, 2, ms.StoreMatchScheduleCallCount, "a rescheduled match should be written")
}

func TestTick_ScopesRequestToTournament(t *testing.T) {
	fake, srv := newFakePandaScore(t)
	p, _, entry := newTickPoller(t, srv.URL)

	require.True(t, p.tick(entry))

	assert.Equal(t, "55", fake.query.Get("filter[tournament_id]"))
}

func TestTick_UnrecoverableStatus_DropsTournament(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			fake, srv := newFakePandaScore(t)
			p, _, entry := newTickPoller(t, srv.URL)
			fake.status = status

			assert.False(t, p.tick(entry))
		})
	}
}

func TestTick_TransientFailure_KeepsTournament(t *testing.T) {
	tests := map[string]struct {
		status int
		body   string
	}{
		"server error":   {http.StatusInternalServerError, ""},
		"malformed body": {http.StatusOK, "not json"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fake, srv := newFakePandaScore(t)
			p, ms, entry := newTickPoller(t, srv.URL)
			fake.status, fake.body = tt.status, tt.body

			assert.True(t, p.tick(entry))
			assert.Equal(t, 0, ms.StoreMatchScheduleCallCount)
		})
	}
}

// endregion

// region scheduleKey tests

func TestScheduleKey_SameMatchesSameKey(t *testing.T) {
	matches := []sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: 1000},
		{Team1: "Team C", Team2: "Team D", EpochTime: 2000},
	}
	assert.Equal(t, scheduleKey(matches), scheduleKey(matches))
}

func TestScheduleKey_OrderIndependent(t *testing.T) {
	a := []sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: 1000},
		{Team1: "Team C", Team2: "Team D", EpochTime: 2000},
	}
	b := []sources.ScheduledMatch{
		{Team1: "Team C", Team2: "Team D", EpochTime: 2000},
		{Team1: "Team A", Team2: "Team B", EpochTime: 1000},
	}
	assert.Equal(t, scheduleKey(a), scheduleKey(b))
}

func TestScheduleKey_DetectsTimeChange(t *testing.T) {
	before := []sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B", EpochTime: 1000}}
	after := []sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B", EpochTime: 9999}}
	assert.NotEqual(t, scheduleKey(before), scheduleKey(after))
}

func TestScheduleKey_DetectsTeamChange(t *testing.T) {
	before := []sources.ScheduledMatch{{Team1: "TBD", Team2: "Team B", EpochTime: 1000}}
	after := []sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B", EpochTime: 1000}}
	assert.NotEqual(t, scheduleKey(before), scheduleKey(after))
}

func TestScheduleKey_EmptySlice(t *testing.T) {
	assert.Equal(t, scheduleKey(nil), scheduleKey([]sources.ScheduledMatch{}))
}

func TestScheduleKey_SameTeam1_SortsByTeam2(t *testing.T) {
	// Exercises the team2 comparison branch in the sort comparator.
	a := []sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Zeta", EpochTime: 1000},
		{Team1: "Team A", Team2: "Alpha", EpochTime: 2000},
	}
	b := []sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Alpha", EpochTime: 2000},
		{Team1: "Team A", Team2: "Zeta", EpochTime: 1000},
	}
	assert.Equal(t, scheduleKey(a), scheduleKey(b))
}

// endregion
