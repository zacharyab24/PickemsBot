package Api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
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

// Function to get match data for a given liquipedia page. Note that the wiki is hard coded to counterstrike. This is the main run function of this file
// Preconditions: Receives string containing liquipedia page (such as BLAST/Major/2025/Austin/Stage_1) and optional params (such as  &section=24 (this is not used in majors))
// Postconditions: None at this stage. TODO: Change return type to be uniform so db can be updated with returned information 
func GetMatchData(page string, optionalParams string) {
	url := fmt.Sprintf("https://liquipedia.net/counterstrike/%s?action=raw%s", page, optionalParams)
	wikitext, err := GetWikitext(url)

	if err != nil {
		fmt.Println("An error occured whilst fetching match2bracketid data: ", err)
		return
	}

	ids, format, err := ExtractMatchListId(wikitext)
	if err != nil {
		fmt.Println("An error occured:", err)
		return
	}

	//Func to get JSON data
	liquipediaDBApiKey := os.Getenv("LIQUIDPEDIADB_API_KEY")
	apiRequestString := "https://api.liquipedia.net/api/v3/match"
	jsonResponse, err := GetLiquipediaMatchData(liquipediaDBApiKey, ids, apiRequestString)
	if err != nil {
		fmt.Println("An error occured whilst fetching match data")
		return
	}
	matchNodes, err := GetMatchNodesFromJson(jsonResponse)
	if err != nil {
		fmt.Println("An error parsing match data", err)
		return
	}

	switch format {
	case "swiss":
		scores, err := CalculateSwissScores(matchNodes)
		if err != nil {
			fmt.Println("An error occured whilst parsing match data")
		}
		for _, team := range scores {
			fmt.Printf("%s: %s\n", team, scores[team])
		}
	case "single-elimination":
		tree, err := GetMatchTree(matchNodes)
		if err != nil {
			fmt.Println("An error occured whilst parsing match data:",err)
			return
		}
		PrintTreeLevelOrder(tree)
	default:
		fmt.Println("unknown format type: ", format)
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
		return nil, "", fmt.Errorf("unknown tournament format detected")
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
		scores[team] = fmt.Sprintf("%s: %d-%d", team, wins[team], loses[team])
	}

	return scores, nil
}

// Function to process match nodes and generate a binary tree for the single elim bracket, where GF is the root node, and the first round matches are the leaves
// Preconditions: Receives slice of match nodes
// Postconditions: Returns root node of the tree, or an error that occurs
func GetMatchTree(matchNodes []MatchNode) (*MatchNode, error) {
	if len(matchNodes) == 0 {
        return nil, fmt.Errorf("no match nodes provided")
    }

    // Map round of each node to its corresponding level in the tree ([1-N] rounds where 0 is the leaves and N is the root)
    levelMap := make(map[int][]*MatchNode)
    for i := range matchNodes {
        round, _, err := extractRoundAndMatchIds(matchNodes[i].Id)
        if err != nil {
            return nil, err
        }
        levelMap[round] = append(levelMap[round], &matchNodes[i])
    }

    // Extract levels from levelMap and sort in decending order (Since we want to work from N to 1)
    levels := make([]int, 0, len(levelMap))
    for round := range levelMap {
        levels = append(levels, round)
    }
    sort.Sort(sort.Reverse(sort.IntSlice(levels)))

	// Iteratively map child nodes to their parents for every tree level
    for i := range len(levels)-1 {
        parents := levelMap[levels[i]]
        children := levelMap[levels[i+1]]

        if len(children) != len(parents)*2 {
            return nil, fmt.Errorf("round %d has %d children, expected %d", 
                levels[i+1], len(children), len(parents)*2)
        }

        for j, parent := range parents {
            parent.Left = children[j*2]
            parent.Right = children[j*2+1]
        }
    }

    // Validate and return root
    rootNodes := levelMap[levels[0]]
    if len(rootNodes) != 1 {
        return nil, fmt.Errorf("expected exactly one root match, got %d", len(rootNodes))
    }
    
    return rootNodes[0], nil
}

// Helper function to get the round and match numbers from a MatchNode Id
// Id is of the form <match2bracketid>_Rxx-Myyy (e.g. RSTxQ88PoQ_R03-M001)
// Preconditions: Receives string containing match id
// Postconditions: Returns round value and match value, or an error
func extractRoundAndMatchIds(id string) (round int, match int, err error) {
	re := regexp.MustCompile(`_R(\d+)-M(\d+)$`)
	matches := re.FindStringSubmatch(id)
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("invalid ID format: %s", id)
	}
	round, _ = strconv.Atoi(matches[1])
	match, _ = strconv.Atoi(matches[2])
	return round, match, nil
}

// PrintTreeLevelOrder prints the tree level by level (breadth-first)
func PrintTreeLevelOrder(root *MatchNode) {
    if root == nil {
        fmt.Println("Empty tree")
        return
    }
    
    fmt.Println("Tournament Tree (Level Order):")
    fmt.Println(strings.Repeat("=", 60))
    
    queue := []*MatchNode{root}
    level := 0
    
    for len(queue) > 0 {
        levelSize := len(queue)
        fmt.Printf("Round %d:\n", level+1)
        
        for i := 0; i < levelSize; i++ {
            node := queue[0]
            queue = queue[1:]
            
            winner := node.Winner
            if winner == "" {
                winner = "TBD"
            }
            
            fmt.Printf("  %s vs %s (Winner: %s)\n", node.Team1, node.Team2, winner)
            
            if node.Left != nil {
                queue = append(queue, node.Left)
            }
            if node.Right != nil {
                queue = append(queue, node.Right)
            }
        }
        fmt.Println()
        level++
    }
}