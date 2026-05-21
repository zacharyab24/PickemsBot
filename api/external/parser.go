/* parser.go
 * Contains the logic used in processing results from external apis and parsing data into formats that other functions can use
 * Authors: Zachary Bower
 */

package external

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// GetMatchNodesFromJSON Function to parse liquipedia match data json and return a slice of MatchNodes
// Preconditions: Receives string containing json match data
// Postconditons: Returns a slice containing MatchNodes or an error that occurs
func GetMatchNodesFromJSON(matchData string) ([]MatchNode, error) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(matchData), &root); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	rawResults, ok := root["result"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'result' field")
	}

	// Iterate over all values in the results list
	var matchNodes []MatchNode
	for _, result := range rawResults {
		node, err := ParseMatchData(result)
		if err != nil {
			fmt.Println("Error creating match node:", err)
			return nil, err
		}
		matchNodes = append(matchNodes, *node)
	}
	return matchNodes, nil
}

// GetScheduledMatchesFromJSON Function to parse liquipedia match data json and return a slice of UpcomingMatch
// Preconditions: Receives string containing json match data
// Postconditons: Returns a slice containing MatchNodes or an error that occurs
func GetScheduledMatchesFromJSON(matchData string) ([]ScheduledMatch, error) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(matchData), &root); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	rawResults, ok := root["result"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'result' field")
	}

	// Iterate over all values in the results list
	var upcomingMatches []ScheduledMatch
	for _, result := range rawResults {
		match, err := ParseScheduledMatches(result)
		if err != nil {
			fmt.Println("Error creating match node:", err)
			return nil, err
		}
		// Match and error will only ever be both nil if there is no upcoming match
		if match == nil {
			continue
		}

		upcomingMatches = append(upcomingMatches, *match)
	}
	return upcomingMatches, nil
}

// ParseMatchData Function to create match nodes from json input
// Preconditions: Receives result interface
// Postconditions: Returns MatchNode pointer populated with match data, or error that occurs
func ParseMatchData(result interface{}) (*MatchNode, error) {
	match, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping match interface")
	}

	// Get match id
	matchIDStr, ok := match["match2id"].(string)
	if !ok {
		return nil, fmt.Errorf("error mapping match2id interface")
	}

	// Check if a match is finished
	finishedStr, ok := match["finished"].(float64)
	if !ok {
		return nil, fmt.Errorf("error mapping finished interface")
	}
	var isFinished bool
	if finishedStr == 0 {
		isFinished = false
	} else if finishedStr == 1 {
		isFinished = true
	} else {
		return nil, fmt.Errorf("non binary result for finished")
	}

	// Get team names
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

	// If the match has finished, get the name of the team that won, else set as TBD
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

// ParseScheduledMatches Function to get scheduled matches from json data
// Preconditions: Receives interface containing json match data
// Postconditons: Returns slice of ScheduledMatch or an error that occurs
func ParseScheduledMatches(result interface{}) (*ScheduledMatch, error) {
	match, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping match interface")
	}

	// Check if a match is finished
	finishedRes, ok := match["finished"].(float64)
	if !ok {
		return nil, fmt.Errorf("error mapping finished interface")
	}
	if finishedRes != 0 && finishedRes != 1 {
		return nil, fmt.Errorf("unexpected value for 'finished': %v (expected 0 or 1)", finishedRes)
	}
	isFinished := finishedRes == 1

	// Get match date
	matchDateStr, ok := match["date"].(string)
	if !ok {
		return nil, fmt.Errorf("error mapping match2id interface")
	}
	// Match dates are in GMT, need to convert to epoch
	layout := "2006-01-02 15:04:05"
	parsedTime, err := time.Parse(layout, matchDateStr)
	if err != nil {
		return nil, err
	}
	epoch := parsedTime.Unix()

	// Get Twitch URL
	streamURL := ""
	if streamMap, ok := match["stream"].(map[string]interface{}); ok {
		if raw, ok := streamMap["twitch"]; ok {
			streamURL, _ = raw.(string)
		} else if raw, ok := streamMap["kick"]; ok {
			streamURL, _ = raw.(string)
		}
	}

	// Get bestOf
	bestOfFloat, ok := match["bestof"].(float64)
	if !ok {
		return nil, fmt.Errorf("bestof field is not a float64")
	}
	bestOf := strconv.FormatFloat(bestOfFloat, 'f', -1, 64)

	// Get team names
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
		EpochTime: epoch,
		BestOf:    bestOf,
		StreamURL: streamURL,
		Finished:  isFinished,
	}, nil

}
