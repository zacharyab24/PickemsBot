/* liquipedia.go
 * Contains HTTP clients and parsers for the Liquipedia MediaWiki and LiquipediaDB APIs
 * Authors: Zachary Bower
 */

package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GetLiquipediaMatchDataByPage fetches all match data for a tournament page from
// the LiquipediaDB /match endpoint using a [[pagename::X]] condition. This
// replaces the older bracket-ID extraction approach: instead of scraping
// match2bracketid values from wikitext and ORing them together, callers simply
// provide the Liquipedia page path (e.g. "PGL/2026/Astana") and all matches
// stored under that page are returned in one query.
//
// Preconditions: valid API key, pagename is a slash-separated Liquipedia path
// Postconditions: returns raw JSON string or an error
func GetLiquipediaMatchDataByPage(apiKey string, pagename string) (string, error) {
	apiURL := "https://api.liquipedia.net/api/v3/match"

	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return "", fmt.Errorf("invalid api url: %w", err)
	}

	params := parsedURL.Query()
	params.Set("limit", "100")
	params.Set("wiki", "counterstrike")
	params.Set("conditions", fmt.Sprintf("[[pagename::%s]]", pagename))
	params.Set("rawstreams", "false")
	params.Set("streamurls", "false")
	parsedURL.RawQuery = params.Encode()

	client := &http.Client{}
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Authorization", fmt.Sprintf("Apikey %s", apiKey))

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("liquipedia api returned status %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// GetLiquipediaMatchData fetches match data from the LiquipediaDB API filtered by match2bracketid.
// Each match2bracketid corresponds to a bracket table on a tournament page.
func GetLiquipediaMatchData(apiKey string, bracketIds []string) (string, error) {
	apiURL := "https://api.liquipedia.net/api/v3/match"

	var conditions []string
	for _, id := range bracketIds {
		conditions = append(conditions, fmt.Sprintf("[[match2bracketid::%s]]", id))
	}

	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	params := parsedURL.Query()
	params.Set("limit", "100")
	params.Set("wiki", "counterstrike")
	params.Set("conditions", strings.Join(conditions, " OR "))
	params.Set("rawstreams", "false")
	params.Set("streamurls", "false")
	parsedURL.RawQuery = params.Encode()

	client := &http.Client{}
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Authorization", fmt.Sprintf("Apikey %s", apiKey))

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// ParseLiquipediaMatches parses a LiquipediaDB match JSON response into a slice of MatchNodes.
func ParseLiquipediaMatches(matchData string) ([]MatchNode, error) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(matchData), &root); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	rawResults, ok := root["result"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'result' field")
	}

	var matchNodes []MatchNode
	for _, result := range rawResults {
		node, err := parseLiquipediaMatch(result)
		if err != nil {
			return nil, err
		}
		matchNodes = append(matchNodes, *node)
	}
	return matchNodes, nil
}

// parseLiquipediaMatch parses a single match entry from LiquipediaDB JSON into a MatchNode.
func parseLiquipediaMatch(result interface{}) (*MatchNode, error) {
	match, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping match interface")
	}

	matchIDStr, ok := match["match2id"].(string)
	if !ok {
		return nil, fmt.Errorf("error mapping match2id interface")
	}

	finished, ok := match["finished"].(float64)
	if !ok {
		return nil, fmt.Errorf("error mapping finished interface")
	}
	var isFinished bool
	switch finished {
	case 0:
		isFinished = false
	case 1:
		isFinished = true
	default:
		return nil, fmt.Errorf("non binary result for finished")
	}

	var teams [2]string
	opponentsRaw, ok := match["match2opponents"].([]interface{})
	if !ok || len(opponentsRaw) != 2 {
		return nil, fmt.Errorf("opponentsRaw requires exactly 2 values, recieved %d", len(opponentsRaw))
	}
	for i := range opponentsRaw {
		team, ok := opponentsRaw[i].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("error mapping team interface")
		}
		name, ok := team["name"].(string)
		if !ok {
			return nil, fmt.Errorf("error mapping team name interface")
		}
		if name == "" {
			name = "TBD"
		}
		teams[i] = name
	}

	winner := "TBD"
	if isFinished {
		winnerStr, ok := match["winner"].(string)
		if !ok {
			return nil, fmt.Errorf("error mapping winner interface")
		}
		winnerIndex, err := strconv.Atoi(winnerStr)
		if err != nil {
			return nil, err
		}
		winner = teams[winnerIndex-1]
	}

	score := ""
	if isFinished {
		bestOfFloat, ok := match["bestof"].(float64)
		if ok {
			if int(bestOfFloat) == 1 {
				// BO1: score is the map score from the single game
				games, ok := match["match2games"].([]interface{})
				if ok && len(games) > 0 {
					if g, ok := games[0].(map[string]interface{}); ok {
						if scores, ok := g["scores"].([]interface{}); ok && len(scores) == 2 {
							s1, ok1 := scores[0].(float64)
							s2, ok2 := scores[1].(float64)
							if ok1 && ok2 {
								score = fmt.Sprintf("%d-%d", int(s1), int(s2))
							}
						}
					}
				}
			} else {
				// BoX: score is the series score from each opponent's score field
				opp1, ok1 := opponentsRaw[0].(map[string]interface{})
				opp2, ok2 := opponentsRaw[1].(map[string]interface{})
				if ok1 && ok2 {
					s1, ok1 := opp1["score"].(float64)
					s2, ok2 := opp2["score"].(float64)
					if ok1 && ok2 {
						score = fmt.Sprintf("%d-%d", int(s1), int(s2))
					}
				}
			}
		}
	}

	section, _ := match["section"].(string)

	return &MatchNode{
		ID:      matchIDStr,
		Team1:   teams[0],
		Team2:   teams[1],
		Winner:  winner,
		Score:   score,
		Section: section,
	}, nil
}

// ParseLiquipediaSchedule parses a LiquipediaDB match JSON response into a slice of ScheduledMatches.
func ParseLiquipediaSchedule(matchData string) ([]ScheduledMatch, error) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(matchData), &root); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	rawResults, ok := root["result"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'result' field")
	}

	var upcomingMatches []ScheduledMatch
	for _, result := range rawResults {
		match, err := parseLiquipediaScheduledMatch(result)
		if err != nil {
			return nil, err
		}
		if match == nil {
			continue
		}
		upcomingMatches = append(upcomingMatches, *match)
	}
	return upcomingMatches, nil
}

// parseLiquipediaScheduledMatch parses a single match entry from LiquipediaDB JSON into a ScheduledMatch.
func parseLiquipediaScheduledMatch(result interface{}) (*ScheduledMatch, error) {
	match, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping match interface")
	}

	finishedRes, ok := match["finished"].(float64)
	if !ok {
		return nil, fmt.Errorf("error mapping finished interface")
	}
	if finishedRes != 0 && finishedRes != 1 {
		return nil, fmt.Errorf("unexpected value for 'finished': %v (expected 0 or 1)", finishedRes)
	}
	isFinished := finishedRes == 1

	matchDateStr, ok := match["date"].(string)
	if !ok {
		return nil, fmt.Errorf("error mapping match2id interface")
	}
	// Liquipedia dates are in GMT; parse to epoch
	parsedTime, err := time.Parse("2006-01-02 15:04:05", matchDateStr)
	if err != nil {
		return nil, err
	}

	streamMap, ok := match["stream"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping stream to map")
	}

	// Prefer Twitch, fall back to Kick
	streamURLRaw, ok := streamMap["twitch"]
	if !ok {
		streamURLRaw, ok = streamMap["kick"]
		if !ok {
			return nil, fmt.Errorf("twitch or kick keys not found in stream map")
		}
	}

	streamURL, ok := streamURLRaw.(string)
	if !ok {
		return nil, fmt.Errorf("stream url is not a string")
	}

	bestOfFloat, ok := match["bestof"].(float64)
	if !ok {
		return nil, fmt.Errorf("bestof field is not a float64")
	}

	var teams [2]string
	opponentsRaw, ok := match["match2opponents"].([]interface{})
	if !ok || len(opponentsRaw) != 2 {
		return nil, fmt.Errorf("opponentsRaw requires exactly 2 values, recieved %d", len(opponentsRaw))
	}
	for i := range opponentsRaw {
		team, ok := opponentsRaw[i].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("error mapping team interface")
		}
		name, ok := team["name"].(string)
		if !ok {
			return nil, fmt.Errorf("error mapping team name interface")
		}
		if name == "" {
			name = "TBD"
		}
		teams[i] = name
	}

	return &ScheduledMatch{
		Team1:     teams[0],
		Team2:     teams[1],
		EpochTime: parsedTime.Unix(),
		BestOf:    strconv.FormatFloat(bestOfFloat, 'f', -1, 64),
		StreamURL: streamURL,
		Finished:  isFinished,
	}, nil
}
