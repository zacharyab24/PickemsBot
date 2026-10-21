package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// vrsStandingsContentsURL is the GitHub contents API listing for the current
// year's VRS global-standings snapshots. Files are named with a yyyy_mm_dd
// suffix, so the last entry alphabetically is always the most recent snapshot.
// Note: the year is hard-coded and must be advanced each season.
const vrsStandingsContentsURL = "https://api.github.com/repos/ValveSoftware/counter-strike_regional_standings/contents/live/2026"

// VRSStanding is one team's row parsed from a VRS standings markdown snapshot.
// StandingsDate is the raw yyyy_mm_dd label from the file header; callers parse
// it into a real date before persisting.
type VRSStanding struct {
	Standing      int
	Points        int
	TeamName      string
	Roster        []string
	StandingsDate string
}

// FetchLatestVRSStandings resolves the most recent VRS standings snapshot on
// GitHub and returns its raw markdown content.
func FetchLatestVRSStandings() (string, error) {
	url, err := latestVRSStandingsURL()
	if err != nil {
		return "", fmt.Errorf("resolve standings url: %w", err)
	}
	return fetchVRSStandingsFile(url)
}

// latestVRSStandingsURL returns the raw download URL for the newest standings file.
func latestVRSStandingsURL() (string, error) {
	resp, err := http.Get(vrsStandingsContentsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d from github contents api", ErrUnrecoverable, resp.StatusCode)
	}

	var contents []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return "", err
	}
	if len(contents) == 0 {
		return "", fmt.Errorf("no standings files listed at %s", vrsStandingsContentsURL)
	}

	url, ok := contents[len(contents)-1]["download_url"].(string)
	if !ok {
		return "", fmt.Errorf("standings listing missing download_url")
	}
	return url, nil
}

// fetchVRSStandingsFile downloads the raw markdown content at the given URL.
func fetchVRSStandingsFile(fileURL string) (string, error) {
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ParseVRSStandings parses a VRS standings markdown file into standings rows.
// The header line ("### Standings as of 2026_05_04<br />") populates StandingsDate.
// A data row looks like: | 1 | 2081 | Vitality | apEX, flameZ, mezii, ropz, ZywOo | [details](...) |
func ParseVRSStandings(content string) []VRSStanding {
	var standingsDate string
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if date, ok := strings.CutPrefix(line, "### Standings as of "); ok {
			standingsDate = strings.TrimSpace(strings.TrimSuffix(date, "<br />"))
			break
		}
	}

	seen := make(map[string]bool)
	var entries []VRSStanding

	for line := range strings.SplitSeq(content, "\n") {
		// data rows start with "| 1 |" etc — skip header, separator, empty lines
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}

		cols := strings.Split(line, "|")
		if len(cols) < 5 {
			continue
		}

		standing, err := strconv.Atoi(strings.TrimSpace(cols[1]))
		if err != nil {
			continue // skips header and separator rows
		}

		teamName := strings.TrimSpace(cols[3])
		if seen[teamName] {
			continue // keep only first (best) occurrence
		}
		seen[teamName] = true

		roster := strings.Split(strings.TrimSpace(cols[4]), ", ")
		points, _ := strconv.Atoi(strings.TrimSpace(cols[2]))

		entries = append(entries, VRSStanding{
			Standing:      standing,
			Points:        points,
			TeamName:      teamName,
			Roster:        roster,
			StandingsDate: standingsDate,
		})
	}

	return entries
}
