package Api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Struct for a binary tree node
// This tree is used for the results of the finals section, or any other single elimination tournament
// Since a tree is top down by design, but a tournament is bottom up, the tree needs to be initialised with placeholder values such as "TBD"
// Then the data can be populated after
type MatchNode struct {
	Id string
	Team1 string
	Team2 string
	Winner string
	Left *MatchNode
	Right *MatchNode
}

// Function to recursively build a tree using MatchNodes
// Preconditions: Receives an int for the tree depth
// Postcondtions: Returns a MatchNode pointer with left and right values as "TBD" for middle nodes, and nil for leaves
func BuildMatchTree(depth int) *MatchNode {
	// Exit condition
	if depth <= 0 {
		return nil
	}

	// Recursively generate left and right branches
	left := BuildMatchTree(depth - 1)
	right := BuildMatchTree(depth - 1)
	
	return &MatchNode{
		Team1: "TBD",
		Team2: "TBD",
		Winner: "TBD",
		Left: left,
		Right: right,
	}
}

// Function to fetch raw wikitext from a given URL. This function does not perform any parsing on the text
// Preconditions: Receives string that contains URL for liquipedia page we wish to parse (e.g. https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Opening_Stage?action=raw)
// Postconditions: Returns string containing raw wiki text and errors
func GetWikitext(url string) (string, error) {
	
	// Create HTTP Request
	client := &http.Client{}
	request, err :=  http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Failed to create request", err)
	}

	// Headers to apply with API requirements
	request.Header.Set("User-Agent", "LiquipediaDataFetcher/1.0")
    request.Header.Set("Accept-Encoding", "gzip")

	response, err := client.Do(request)
	if err != nil {
		fmt.Println("Request failed: ", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		fmt.Printf("Failed to fetch page. Status code: %d/n", response.StatusCode)
		return "", err
	}

	// Get wiki text from response
	var body []byte
	if response.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(response.Body)
		if err != nil {
			fmt.Println("Failed to create gzip reader: ", err)
			return "", err
		}
		defer reader.Close()
		body, err = io.ReadAll(reader)
		if err != nil {
			return "", err
		}
	} else {
		body, err = io.ReadAll(response.Body)
	}

	if err != nil {
		fmt.Println("Failed to read response body:",err)
		return "", err
	}

	return string(body), err
}

// Function to parse wiki text and extract `Matchlist` id
// Preconditions: Receives string containing wiki text
// Postconditions: Returns string slice containing id's present in input text, or error if an invalid tournament format is detected or no results are found
func ExtractMatchListId(wikitext string) ([]string, error) {
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
		return nil, fmt.Errorf("unknown tournament format detected")
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
		return nil, fmt.Errorf("no ids found")
	}

	return ids, nil
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

// Function to get match data from liquipediaDB filtered by `match2bracketid`. Each match2bracketid should give a table in the "Detailed Results" section for a round of a tournament
// e.g. For the URL https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Opening_Stage, we should be fetching the data for each of the matches in all 9 tables
// Preconditions: Receives string containing liquipediadb api key, Receives url containing tournament page, receives string slice containing match2bracketid's
// Postconditons: Returns the match data json as a string or errors
func GetLiquipediaMatchData(apiKey string, bracketIds []string, tournamentUrl string) (string, error) {
	
	// Format match2bracketids for URL params
	var conditions []string
	for _, id := range bracketIds {
		conditions = append(conditions, fmt.Sprintf("[[match2bracketid::%s]]", id))
	}
	conditionString := strings.Join(conditions, " OR ")

	// Convert tournalmentUrl string into url so we can add params
	parsedUrl, err := url.Parse(tournamentUrl)
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
		fmt.Println("Request failed: ", err)
	}
	defer response.Body.Close()

	// Check if we got a HTTP 200 response, if not an error has occured
	if response.StatusCode != http.StatusOK {
		fmt.Printf("Failed to fetch page. Status code: %d/n", response.StatusCode)
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
		scores[team] = fmt.Sprintf("%s: %d-%d", team, wins[team], loses[team])
	}

	return scores, nil
}

// Function to process match nodes and generate a binary tree for the single elim bracket
// Preconditions: Receives slice of match nodes
// Postconditions: Returns root node of the tree
func GetMatchTree(matchNodes []MatchNode) (*MatchNode, error) {

	// Generate MatchTree
	// The tree will have len(rawResults) matches (numMatches), which is a depth of ⌈ log_2(numMatches) ⌉
	treeDepth := int(math.Ceil(math.Log2(float64(len(matchNodes)))))
	tree := BuildMatchTree(treeDepth)
	
	// Populate tree values
	for _, node := range matchNodes {
		fmt.Printf("Id: %s\nTeam1: %s\nTeam2: %s\nWinner: %s\n\n", node.Id, node.Team1, node.Team2, node.Winner)		
	}

	return tree, nil
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

