package sources

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// PandaScore tournament-listing endpoints. Both use page[size]=100 so the full
// active set fits in one call: the catalog finished-diff treats any DB tournament
// missing from the active set as finished, so a truncated page would wrongly mark
// the overflow as finished.
const (
	pandaScoreUpcomingTournamentsURL = "https://api.pandascore.co/csgo/tournaments/upcoming?filter[tier]&page[size]=100"
	pandaScoreRunningTournamentsURL  = "https://api.pandascore.co/csgo/tournaments/running?filter[tier]&page[size]=100"
)

// PandaScoreTournament is a tournament (stage) as returned by the PandaScore
// tournaments endpoints. In PandaScore's model each stage of a series is its own
// tournament; Round is the stage name (e.g. "Playoffs") and SerieID groups the
// stages of one event.
type PandaScoreTournament struct {
	ID      int    `json:"id"`
	Round   string `json:"name"`
	SerieID int    `json:"serie_id"`

	League struct {
		Name string `json:"name"`
	} `json:"league"`

	Serie struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	} `json:"serie"`
}

// GetUpcomingPandaScoreTournaments fetches the upcoming tournaments for the catalog.
func GetUpcomingPandaScoreTournaments(apiKey string) ([]PandaScoreTournament, error) {
	return getPandaScoreTournaments(apiKey, pandaScoreUpcomingTournamentsURL)
}

// GetRunningPandaScoreTournaments fetches the currently-running tournaments for the catalog.
func GetRunningPandaScoreTournaments(apiKey string) ([]PandaScoreTournament, error) {
	return getPandaScoreTournaments(apiKey, pandaScoreRunningTournamentsURL)
}

func getPandaScoreTournaments(apiKey, url string) ([]PandaScoreTournament, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("authorization", "Bearer "+apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized ||
		res.StatusCode == http.StatusForbidden ||
		res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: status %d from %s", ErrUnrecoverable, res.StatusCode, url)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", res.StatusCode, url)
	}

	var tournaments []PandaScoreTournament
	if err := json.NewDecoder(res.Body).Decode(&tournaments); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return tournaments, nil
}

// DisplayName builds the human-readable tournament name shown in the /config
// picklist, preferring the serie name and falling back to the league name.
func (t PandaScoreTournament) DisplayName() string {
	if t.Serie.Name != "" {
		return fmt.Sprintf("%s %s", t.League.Name, t.Serie.Name)
	}

	if t.Serie.FullName != "" {
		return fmt.Sprintf("%s %s", t.League.Name, t.Serie.FullName)
	}

	return t.League.Name
}
