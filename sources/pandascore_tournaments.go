//TODO: implement format detection to be used by tournament fetcher

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
	pandaScoreBracketURLTemplate     = "https://api.pandascore.co/tournaments/%d/brackets?page[size]=100"
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

// BracketMatch is one match from the PandaScore brackets endpoint. Only the
// fields needed for format detection and team counting are decoded.
type BracketMatch struct {
	ID              int               `json:"id"`
	PreviousMatches []BracketEdge     `json:"previous_matches"`
	Opponents       []BracketOpponent `json:"opponents"`
}

// BracketEdge links a match to a feeder match: Type ("winner" or "loser")
// identifies which prior-match outcome advances into it. A "loser" edge means a
// lower bracket exists, i.e. double-elimination.
type BracketEdge struct {
	FromMatchID int    `json:"match_id"`
	Type        string `json:"type"` // "winner" or "loser"
}

// BracketOpponent wraps one side of a match. Opponent is nil for a TBD slot
// (a not-yet-decided seed), so callers must nil-check before reading it.
type BracketOpponent struct {
	Opponent *BracketTeam `json:"opponent"`
}

// BracketTeam is the minimal team identity carried on a bracket match opponent.
// acronym is frequently null in the API, so ID is the reliable identity key.
type BracketTeam struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CountTeams returns the number of distinct teams across a bracket's matches,
// keyed on opponent ID. TBD/nil opponents are skipped. This is reliable for the
// group-stage case that needs it: a round-robin has every pairing up front, and
// a swiss's round 1 includes every team, so both surface the full field even
// before play. Elimination brackets have TBD later rounds, but they are
// classified by their edges and never reach the team-count check.
func CountTeams(matches []BracketMatch) int {
	seen := make(map[int]struct{})
	for _, m := range matches {
		for _, o := range m.Opponents {
			if o.Opponent != nil && o.Opponent.ID != 0 {
				seen[o.Opponent.ID] = struct{}{}
			}
		}
	}
	return len(seen)
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

// GetPandaScoreBracket fetches the bracket (the match feeder graph) for a
// PandaScore tournament, used to classify its format. Returns ErrUnrecoverable
// on 401/403/404.
func GetPandaScoreBracket(apiKey string, tournamentID int) ([]BracketMatch, error) {
	url := fmt.Sprintf(pandaScoreBracketURLTemplate, tournamentID)
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

	var bracket []BracketMatch
	if err := json.NewDecoder(res.Body).Decode(&bracket); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return bracket, nil
}
