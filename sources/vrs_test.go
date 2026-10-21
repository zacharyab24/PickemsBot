package sources

import (
	"net/http"
	"testing"

	"pickems-bot/internal/testutil"
)

func TestParseVRSStandings(t *testing.T) {
	content := `# Counter-Strike Regional Standings

### Standings as of 2026_05_04<br />

| Rank | Points | Team | Players | Details |
| --- | --- | --- | --- | --- |
| 1 | 2081 | Vitality | apEX, flameZ, mezii, ropz, ZywOo | [details](url) |
| 2 | 1800 | FaZe | karrigan, rain, frozen, ropz, broky | [details](url) |
| 2 | 1800 | FaZe | karrigan, rain, frozen, ropz, broky | [details](url) |
`

	got := ParseVRSStandings(content)

	if len(got) != 2 {
		t.Fatalf("expected 2 entries (duplicate FaZe row dropped), got %d: %+v", len(got), got)
	}

	if got[0].StandingsDate != "2026_05_04" {
		t.Errorf("standings date = %q, want %q", got[0].StandingsDate, "2026_05_04")
	}

	first := got[0]
	if first.Standing != 1 || first.Points != 2081 || first.TeamName != "Vitality" {
		t.Errorf("first entry = %+v, want standing 1 / points 2081 / Vitality", first)
	}
	if len(first.Roster) != 5 {
		t.Errorf("Vitality roster size = %d, want 5 (%+v)", len(first.Roster), first.Roster)
	}
}

func TestParseVRSStandings_NoDataRows(t *testing.T) {
	got := ParseVRSStandings("### Standings as of 2026_05_04<br />\n\nno table here\n")
	if len(got) != 0 {
		t.Errorf("expected no entries, got %d", len(got))
	}
}

func TestFetchLatestVRSStandings_DownloadsNewestFile(t *testing.T) {
	testutil.RouteDefaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/ValveSoftware/counter-strike_regional_standings/contents/live/2026":
			_, _ = w.Write([]byte(`[{"download_url": "https://raw.githubusercontent.com/a.md"}, {"download_url": "https://raw.githubusercontent.com/b.md"}]`))
		case "/b.md":
			_, _ = w.Write([]byte("newest"))
		default:
			t.Errorf("unexpected request to %s", r.URL.Path)
		}
	})

	got, err := FetchLatestVRSStandings()
	if err != nil {
		t.Fatal(err)
	}
	if got != "newest" {
		t.Errorf("got %q, want the newest snapshot", got)
	}
}

func TestFetchLatestVRSStandings_BadListing(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"rate limited":         func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) },
		"empty listing":        func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) },
		"missing download url": func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[{"name": "x.md"}]`)) },
		"malformed":            func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{`)) },
	}
	for name, h := range tests {
		t.Run(name, func(t *testing.T) {
			testutil.RouteDefaultClient(t, h)
			if _, err := FetchLatestVRSStandings(); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
