/* liquipedia.go
 * Contains the logic used to fetch data LiquipediaDB api and process the results
 * Authors: Zachary Bower
 */

package external

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"pickems-bot/api/shared"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Function to get match data from liquipediaDB filtered by `match2bracketid`. Each match2bracketid should give a table in the "Detailed Results" section for a round of a tournament
// e.g. For the URL https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Opening_Stage, we should be fetching the data for each of the matches in all 9 tables
// Preconditions: Receives string containing liquipediadb api key, Receives url containing tournament page, receives string slice containing match2bracketid's
// Postconditons: Returns the match data json as a string or errors
func GetLiquipediaMatchData(apiKey string, bracketIds []string) (string, error) {
	apiUrl := "https://api.liquipedia.net/api/v3/match"

	// Format match2bracketids for URL params
	var conditions []string
	for _, id := range bracketIds {
		conditions = append(conditions, fmt.Sprintf("[[match2bracketid::%s]]", id))
	}
	conditionString := strings.Join(conditions, " OR ")

	// Convert tournalmentUrl string into url so we can add params
	parsedUrl, err := url.Parse(apiUrl)
	if err != nil {
		fmt.Println("Invalid url:",err)
		return "", err
	}

	// Set URL parameters
	params := parsedUrl.Query()
	params.Set("limit", "100")
	params.Set("wiki", "counterstrike")
	params.Set("conditions", conditionString)
	params.Set("rawstreams", "false")
	params.Set("streamurls", "false")
	parsedUrl.RawQuery = params.Encode()

	// Create HTTP Request
	client := &http.Client{}
	request, err :=  http.NewRequest("GET", parsedUrl.String(), nil)
	if err != nil {
		fmt.Println("Failed to create request", err)
		return "", err;
	}

	// Apply auth header to request
	request.Header.Set("Authorization", fmt.Sprintf("Apikey %s", apiKey))

	// Send request
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	// Check if we got a HTTP 200 response, if not an error has occured
	if response.StatusCode != http.StatusOK {
		fmt.Printf("Failed to fetch page. Status code: %d\n", response.StatusCode)
		return "", err
	}

	// Extract body from reponse and return it
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Failed to read body response:", err)
		return "", err
	}

	return string(body), nil
}

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
			fmt.Println("Error creating match node:",err)
			return nil, err
		}
		matchNodes = append(matchNodes, *node)	
	}
	return matchNodes, nil
}

// Function to parse liquipedia match data json and return a slice of UpcomingMatch
// Preconditions: Receives string containing json match data
// Postconditons: Returns a slice containing MatchNodes or a error that occurs
func GetUpcomingMatchesFromJson(matchData string) ([]UpcomingMatch, error) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(matchData), &root); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	rawResults, ok := root["result"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'result' field")
	}

	// Iterate over all values in the results list
	var upcomingMatches []UpcomingMatch
	for _, result := range rawResults {
		match, err := ParseUpcomingMatches(result)
		if err != nil {
			fmt.Println("Error creating match node:",err)
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
		Id: matchIdStr,
		Team1: teams[0],
		Team2: teams[1],
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
		if ! slices.Contains(teams, node.Team2) {
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
func getRoundNames(numMatches int) ([]string, error){
	// Find the number of rounds, this way we can make sure the name mapping is correct
	// numRounds = log_2 (numMatches + 1) since there are always n-1 matches for a single elim tournament with n teams
	numRounds := int(math.Ceil(math.Log2(float64(numMatches+1))))

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

// Function to get upcoming matches from json data
// Preconditions: Receives interface containing json match data
// Postconditons: Returns slice of UpcomingMatch or an error that occurs
func ParseUpcomingMatches(result interface{}) (*UpcomingMatch, error) {
	match, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error mapping match interface")
	}

	// Check if a match is finished
	finishedStr, ok := match["finished"].(float64)
	if !ok {
		return nil, fmt.Errorf("error mapping finished interface")
	}
	
	// If match has finished, return nil for this node
	if finishedStr != 0 {
		return nil, nil
	} 

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
	
	return &UpcomingMatch{
		Team1: teams[0],
		Team2: teams[1],
		EpochTime: epoch,
		BestOf: bestOf,
		StreamUrl: twitchUrl,
	}, nil 

}