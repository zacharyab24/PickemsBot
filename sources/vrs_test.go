package sources

import "testing"

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
