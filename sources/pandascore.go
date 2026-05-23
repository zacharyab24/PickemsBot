package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrUnrecoverable signals that the API returned an error that will not resolve on retry
// (e.g. bad API key, unknown series ID). The poller should stop when it sees this.
var ErrUnrecoverable = errors.New("unrecoverable api error")

// GetPandaScoreMatches fetches all matches for a given series from the PandaScore API.
// Returns the raw JSON response body as a string.
func GetPandaScoreMatches(apiKey string, seriesID int) (string, error) {
	apiURL := "https://api.pandascore.co/csgo/matches"

	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	params := parsedURL.Query()
	params.Set("filter[serie_id]", strconv.Itoa(seriesID))
	params.Set("filter[status]", "finished,running,not_started")
	params.Set("per_page", "50")
	parsedURL.RawQuery = params.Encode()

	client := &http.Client{}
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized ||
		response.StatusCode == http.StatusForbidden ||
		response.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w: status %d", ErrUnrecoverable, response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// ParsePandaScoreMatches parses a PandaScore matches JSON response into a slice of MatchNodes.
func ParsePandaScoreMatches(matchData string) ([]MatchNode, error) {
	var raw []interface{}
	if err := json.Unmarshal([]byte(matchData), &raw); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	var matchNodes []MatchNode
	for _, result := range raw {
		node, err := parsePandaScoreMatch(result)
		if err != nil {
			return nil, err
		}
		matchNodes = append(matchNodes, *node)
	}
	return matchNodes, nil
}

func parsePandaScoreMatch(result interface{}) (*MatchNode, error) {
	match, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping match interface")
	}

	idFloat, ok := match["id"].(float64)
	if !ok {
		return nil, fmt.Errorf("error mapping id")
	}
	id := strconv.Itoa(int(idFloat))

	section, _ := match["name"].(string)

	status, ok := match["status"].(string)
	if !ok {
		return nil, fmt.Errorf("error mapping status")
	}
	isFinished := status == "finished"

	// Extract team names and IDs from opponents
	var teams [2]string
	var teamIDs [2]float64
	opponents, _ := match["opponents"].([]interface{})
	for i := 0; i < 2; i++ {
		teams[i] = "TBD"
		if i < len(opponents) {
			if slot, ok := opponents[i].(map[string]interface{}); ok {
				if opp, ok := slot["opponent"].(map[string]interface{}); ok {
					if name, ok := opp["name"].(string); ok && name != "" {
						teams[i] = name
					}
					teamIDs[i], _ = opp["id"].(float64)
				}
			}
		}
	}

	winner := "TBD"
	if isFinished {
		if w, ok := match["winner"].(map[string]interface{}); ok {
			if name, ok := w["name"].(string); ok {
				winner = name
			}
		}
	}

	// Cross-reference results by team_id to get score in Team1-Team2 order
	score := ""
	if isFinished {
		scores := map[float64]int{}
		if results, ok := match["results"].([]interface{}); ok {
			for _, r := range results {
				if entry, ok := r.(map[string]interface{}); ok {
					tid, _ := entry["team_id"].(float64)
					s, _ := entry["score"].(float64)
					scores[tid] = int(s)
				}
			}
		}
		if len(scores) == 2 {
			score = fmt.Sprintf("%d-%d", scores[teamIDs[0]], scores[teamIDs[1]])
		}
	}

	return &MatchNode{
		ID:      id,
		Team1:   teams[0],
		Team2:   teams[1],
		Winner:  winner,
		Score:   score,
		Section: section,
		Status:  status,
	}, nil
}

// ParsePandaScoreSchedule parses a PandaScore matches JSON response into a slice of ScheduledMatches.
func ParsePandaScoreSchedule(matchData string) ([]ScheduledMatch, error) {
	var raw []interface{}
	if err := json.Unmarshal([]byte(matchData), &raw); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	var scheduledMatches []ScheduledMatch
	for _, result := range raw {
		match, err := parsePandaScoreScheduledMatch(result)
		if err != nil {
			return nil, err
		}
		scheduledMatches = append(scheduledMatches, *match)
	}
	return scheduledMatches, nil
}

func parsePandaScoreScheduledMatch(result interface{}) (*ScheduledMatch, error) {
	match, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping match interface")
	}

	status, ok := match["status"].(string)
	if !ok {
		return nil, fmt.Errorf("error mapping status")
	}
	isFinished := status == "finished"

	scheduledAt, ok := match["scheduled_at"].(string)
	if !ok {
		return nil, fmt.Errorf("error mapping scheduled_at")
	}
	parsedTime, err := time.Parse(time.RFC3339, scheduledAt)
	if err != nil {
		return nil, fmt.Errorf("error parsing scheduled_at: %w", err)
	}

	bestOfFloat, ok := match["number_of_games"].(float64)
	if !ok {
		return nil, fmt.Errorf("error mapping number_of_games")
	}

	// Extract team names from opponents
	var teams [2]string
	opponents, _ := match["opponents"].([]interface{})
	for i := 0; i < 2; i++ {
		teams[i] = "TBD"
		if i < len(opponents) {
			if slot, ok := opponents[i].(map[string]interface{}); ok {
				if opp, ok := slot["opponent"].(map[string]interface{}); ok {
					if name, ok := opp["name"].(string); ok && name != "" {
						teams[i] = name
					}
				}
			}
		}
	}

	// Find the main official stream URL; prefer Twitch over other platforms.
	// Scan all main+official streams rather than stopping at the first match,
	// so a Twitch entry later in the list beats an earlier Kick entry.
	streamURL := ""
	if streams, ok := match["streams_list"].([]interface{}); ok {
		for _, s := range streams {
			stream, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			main, _ := stream["main"].(bool)
			official, _ := stream["official"].(bool)
			if !main || !official {
				continue
			}
			raw, _ := stream["raw_url"].(string)
			if strings.Contains(raw, "twitch.tv") {
				streamURL = raw
				break // Twitch found — no need to look further
			}
			if streamURL == "" {
				streamURL = raw // first main+official as fallback
			}
		}
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
