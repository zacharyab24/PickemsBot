/* parser.go
 * Contains the logic used in processing results from external apis and parsing data into formats that other functions can use
 * Authors: Zachary Bower
 */

package external

import (
	"encoding/json"
	"fmt"
	"math"
	"pickems-bot/api/shared"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Function to parse liquipedia match data json and return a slice of MatchNodes
// Preconditions: Receives string containing json match data
// Postconditons: Returns a slice containing MatchNodes or a error that occurs
func GetMatchNodesFromJson(matchData string) ([]MatchNode, error) {
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

// Function to parse liquipedia match data json and return a slice of UpcomingMatch
// Preconditions: Receives string containing json match data
// Postconditons: Returns a slice containing MatchNodes or a error that occurs
func GetScheduledMatchesFromJson(matchData string) ([]ScheduledMatch, error) {
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

// Function to create match nodes from json input
// Preconditions: Receives result interface
// Postconditions: Returns MatchNode pointer populated with match data, or error that occur
func ParseMatchData(result interface{}) (*MatchNode, error) {
	match, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping match interface")
	}

	// Get match id
	matchIdStr, ok := match["match2id"].(string)
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

	return &MatchNode{
		Id:     matchIdStr,
		Team1:  teams[0],
		Team2:  teams[1],
		Winner: winner,
	}, nil
}

// Function to process match nodes and calculate swiss score
// Preconditions: Receives slice of match nodes
// Postconditions: Returns map[string]string containing teams:scores
func CalculateSwissScores(matchNodes []MatchNode) (map[string]string, error) {
	var teams []string
	wins := make(map[string]int)
	loses := make(map[string]int)

	for i := range matchNodes {
		node := matchNodes[i]

		// Check if teams are in teams slice
		if !slices.Contains(teams, node.Team1) {
			teams = append(teams, node.Team1)
		}
		if !slices.Contains(teams, node.Team2) {
			teams = append(teams, node.Team2)
		}

		// Update win and loss maps
		if node.Winner == "TBD" {
			continue
		}
		if node.Winner == node.Team1 {
			wins[node.Team1]++
			loses[node.Team2]++
		} else if node.Winner == node.Team2 {
			wins[node.Team2]++
			loses[node.Team1]++
		} else {
			// Unexpected winner value skip
			continue
		}

	}

	scores := make(map[string]string)
	for _, team := range teams {
		//Skip any placeholder teams names
		if team == "TBD" {
			continue
		}
		scores[team] = fmt.Sprintf("%d-%d", wins[team], loses[team])
	}

	return scores, nil
}

// Function to process a slice of match nodes and return a map of team name : TeamProgress. Used for processing of Elimination stage matches
// Preconditions: Receives slice of match nodes. Limitation of upto 5 rounds of matches (best of 32 tournament) but can be easily extended
// Postconditions: Returns map of team name to TeamProgress (stage and status)
func GetEliminationResults(matchNodes []MatchNode) (map[string]shared.TeamProgress, error) {
	if len(matchNodes) == 0 {
		return nil, fmt.Errorf("at least one match required, recieved 0")
	}

	// Ordered slice of rounds where [0] is the last match (grand final), and [n] is the first. This is where the limitation of 32 comes from
	rounds, err := getRoundNames(len(matchNodes))
	if err != nil {
		return nil, err
	}

	results := make(map[string]shared.TeamProgress)

	for _, match := range matchNodes {
		roundNum, _, err := ExtractRoundAndMatchIds(match.Id)
		if err != nil {
			return nil, err
		}

		// Safely resolve stage and rank
		round := fmt.Sprintf("Round %d", roundNum)
		rank := -1
		index := len(rounds) - roundNum
		if index >= 0 && index < len(rounds) {
			round = rounds[index]
			rank = roundNum
		}
		// Assign initial progress (pending) for each team
		for _, team := range []string{match.Team1, match.Team2} {
			if team != "" {
				existing, ok := results[team]
				if !ok || rank > getRoundIndex(existing.Round, rounds) {
					results[team] = shared.TeamProgress{
						Round:  round,
						Status: "pending",
					}
				}
			}
		}

		// If there's a winner, update winner/loser status
		if match.Winner != "TBD" && match.Winner != "" {
			results[match.Winner] = shared.TeamProgress{
				Round:  round,
				Status: "advanced",
			}

			// Determine loser
			var loser string
			if match.Team1 == match.Winner {
				loser = match.Team2
			} else {
				loser = match.Team1
			}
			if loser != "" {
				results[loser] = shared.TeamProgress{
					Round:  round,
					Status: "eliminated",
				}
			}
		}
	}
	return results, nil
}

// Helper function to get the index of a round from its name. Used in GetEliminationResults
// Preconditions: Receives a string, and slice of strings
// Postconditions: Returns the index of that string in the slice, or -1 if not found
func getRoundIndex(round string, rounds []string) int {
	for i, name := range rounds {
		if name == round {
			return len(rounds) - i
		}
	}
	return -1 // Unknown stage
}

// Helper function to get the names of rounds for a single elim tournament, this is a hardcoded list with a limit of 32 matches
// Preconditions Receives int containing number of matches in this tournament
// Postconditions: Returns string slice containing round names, or error if it occurs
func getRoundNames(numMatches int) ([]string, error) {
	// Find the number of rounds, this way we can make sure the name mapping is correct
	// numRounds = log_2 (numMatches + 1) since there are always n-1 matches for a single elim tournament with n teams
	numRounds := int(math.Ceil(math.Log2(float64(numMatches + 1))))

	// Hardcoded slice of round names
	roundNames := []string{
		"Grand Final",
		"Semi Final",
		"Quarter Final",
		"Best of 16",
		"Best of 32",
	}

	if numRounds > len(roundNames) {
		return nil, fmt.Errorf("unsupported depth: only up to %d rounds supported", len(roundNames))
	}

	return roundNames[:numRounds], nil
}

// Helper function to get the round and match numbers from a MatchNode Id
// Id is of the form <match2bracketid>_Rxx-Myyy (e.g. RSTxQ88PoQ_R03-M001)
// Preconditions: Receives string containing match id
// Postconditions: Returns round value and match value, or an error
func ExtractRoundAndMatchIds(id string) (round int, match int, err error) {
	re := regexp.MustCompile(`_R(\d+)-M(\d+)$`)
	matches := re.FindStringSubmatch(id)
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("invalid ID format: %s", id)
	}
	round, _ = strconv.Atoi(matches[1])
	match, _ = strconv.Atoi(matches[2])
	return round, match, nil
}

// Function to get scheduled matches from json data
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

	// // If match has finished, return nil for this node
	// if finishedStr != 0 {
	// 	return nil, nil
	// }

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
	streamMap, ok := match["stream"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping stream to map")
	}

	twitchUrlRaw, ok := streamMap["twitch"]
	if !ok {
		return nil, fmt.Errorf("twitch key not found in stream")
	}

	twitchUrl, ok := twitchUrlRaw.(string)
	if !ok {
		return nil, fmt.Errorf("twitch url is not a string")
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
		StreamUrl: twitchUrl,
		Finished:  isFinished,
	}, nil

}

// Function to parse wiki text and extract `Matchlist` id
// Preconditions: Receives string containing wiki text
// Postconditions: Returns string slice containing id's present in input text and tournament format, or error if an invalid tournament format is detected or no results are found
func ExtractMatchListId(wikitext string) ([]string, string, error) {
	ids := []string{}
	format := DetectTournamentFormat(wikitext)
	var re *regexp.Regexp

	// Set regex for tournament format
	switch format {
	case "swiss":
		re = regexp.MustCompile(`(?s)\{\{\s*Matchlist\s*\|([^}]*)\}\}`) // {{Matchlist ...}} templates used in swiss tournaments
	case "single-elimination":
		re = regexp.MustCompile(`(?s)\{\{\s*Bracket\s*\|([^}]*)\}\}`) // {{ShowBracket ...}} templates used in swiss tournaments
	default:
		return nil, "", fmt.Errorf("unknown tournament format detected %s", format)
	}

	// Find regex matches
	matches := re.FindAllStringSubmatch(wikitext, -1)
	for _, match := range matches {
		paramsText := match[1]

		// Parse pipe ("|") seperated key value pairs from template
		parts := strings.Split(paramsText, "|")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "id=") {
				id := strings.TrimSpace(strings.TrimPrefix(part, "id="))

				// Remove trailing html comments (some times occurs in single elim data)
				reComment := regexp.MustCompile(`<!--.*?-->`)
				id = reComment.ReplaceAllString(id, "")
				id = strings.TrimSpace(id)

				if id != "" {
					ids = append(ids, id)
				}
				break // No need to parse more params
			}
		}
	}

	if len(ids) == 0 {
		return nil, "", fmt.Errorf("no ids found")
	}
	return ids, format, nil
}

// Function to determine the format of a tournament from a given wiki text, e.g. swiss, single-elimination
// Preconditions: Receives string containing the raw wikitext
// Preconditions: Returns string containing the format of the tournament
func DetectTournamentFormat(wikitext string) string {

	// Regex to find ==Format== section in wikitext
	re := regexp.MustCompile(`(?s)==\s*Format\s*==\s*(.*)`)
	results := re.FindStringSubmatch(wikitext)

	if len(results) > 1 {
		formatSection := results[1] // format is listed on the second line of the format section in wikitext
		switch {
		case strings.Contains(strings.ToLower(formatSection), "swiss") && strings.Contains(strings.ToLower(formatSection), "single-elimination"):
			// This case occurs when both styles are on a singular page. This doesnt occur during the major and is just here for testing
			return "single-elimination"
		case strings.Contains(strings.ToLower(formatSection), "swiss"):
			return "swiss"
		case strings.Contains(strings.ToLower(formatSection), "single-elimination"):
			return "single-elimination"
		default:
			return "unknown"
		}
	}
	return "unknown"
}
