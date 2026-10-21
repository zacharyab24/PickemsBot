package ingest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"pickems-bot/app"
	"pickems-bot/internal/testutil"
	"pickems-bot/store"

	"github.com/stretchr/testify/assert"
)

const vrsSnapshot = `### Standings as of 2026_05_04<br />

| Rank | Points | Team | Players | Details |
| --- | --- | --- | --- | --- |
| 1 | 2081 | Vitality | apEX, ropz | [details](url) |
| 2 | 1800 | FaZe | karrigan, broky | [details](url) |
`

// vrsGitHub serves a contents listing whose newest entry points at snapshot.
func vrsGitHub(snapshot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/ValveSoftware/counter-strike_regional_standings/contents/live/2026":
			_, _ = w.Write([]byte(`[
				{"download_url": "https://raw.githubusercontent.com/old.md"},
				{"download_url": "https://raw.githubusercontent.com/newest.md"}
			]`))
		case "/newest.md":
			_, _ = w.Write([]byte(snapshot))
		default:
			http.NotFound(w, r)
		}
	}
}

func TestVRSSync_RunOnce_SyncsNewestSnapshot(t *testing.T) {
	testutil.RouteDefaultClient(t, vrsGitHub(vrsSnapshot))
	ms := app.NewMockStore("swiss", "test_round")

	NewVRSSync(app.NewTestApp(ms), 0, nil).runOnce(context.Background())

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, []store.VRSEntry{
		{Standing: 1, Points: 2081, TeamName: "Vitality", Roster: []string{"apEX", "ropz"}, StandingsDate: date},
		{Standing: 2, Points: 1800, TeamName: "FaZe", Roster: []string{"karrigan", "broky"}, StandingsDate: date},
	}, ms.SyncedStandings)
}

func TestVRSSync_RunOnce_SkipsUnusableSnapshots(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"listing unavailable": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) },
		"no standings rows":   vrsGitHub("### Standings as of 2026_05_04<br />\n"),
		"unparseable date":    vrsGitHub("### Standings as of May 4<br />\n\n| 1 | 2081 | Vitality | apEX | x |\n"),
	}
	for name, h := range tests {
		t.Run(name, func(t *testing.T) {
			testutil.RouteDefaultClient(t, h)
			ms := app.NewMockStore("swiss", "test_round")

			NewVRSSync(app.NewTestApp(ms), 0, nil).runOnce(context.Background())

			assert.Nil(t, ms.SyncedStandings)
		})
	}
}
