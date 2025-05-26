package Api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Function to fetch raw wikitext from a given URL. This function does not perform any parsing on the text
// Preconditions: Recieves string that contains URL for liquipedia page we wish to parse (e.g. https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Opening_Stage?action=raw)
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
// Preconditions: Recieves string containing wiki text  
// Postconditions: Returns string slice containing id's present in input text
func ExtractMatchListId(wikitext string) []string {
	ids := []string{}

	//Regex to match {{Matchlist ...}} templates
	re := regexp.MustCompile(`(?s)\{\{\s*Matchlist\s*\|([^}]*)\}\}`)

	matches := re.FindAllStringSubmatch(wikitext, -1)
	for _, match := range matches {
		paramsText := match[1]

		// Parse pipe ("|") seperated key value pairs from template
		parts := strings.Split(paramsText, "|")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "id=") {
				id := strings.TrimSpace(strings.TrimPrefix(part, "id="))
				if id != "" {
					ids = append(ids, id)
				}
				break // No need to parse more params
			}
		}
	}
	return ids
}

// Function to get match data from liquipediaDB filtered by `match2bracketid`. Each match2bracketid should give a table in the "Detailed Results" section for a round of a tournament
// e.g. For the URL https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Opening_Stage, we should be fetching the data for each of the matches in all 9 tables
// Preconditions: Recieves string containing liquipediadb api key, recieves url containing tournament page, receives string slice containing match2bracketid's
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

// Function to parse liquipedia match data and extract scores for each team
// Preconditions: Recieves string containing json match data
// Postconditions: Returns map[string]string containing teams:scores
func GetScoresFromJson(matchData string) (map[string]string, error) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(matchData), &root); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	resultsRaw, ok := root["result"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'result' field")
	}

	// Initialize teams, wins, and loses data structures
	var teams []string
	wins := make(map[string]int)
	loses := make(map[string]int)

	contains := func(slice []string, str string) bool {
		for _, v := range slice {
			if v == str {
				return true
			}
		}
		return false
	}

	for _, item := range resultsRaw {
		match, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		opponentsRaw, ok := match["match2opponents"].([]interface{})
		if !ok || len(opponentsRaw) != 2 {
			continue
		}

		winnerStr, ok := match["winner"].(string)
		if !ok {
			continue
		}

		// Add teams to slice and initialize wins/loses maps
		for _, teamRaw := range opponentsRaw {
			teamData, ok := teamRaw.(map[string]interface{})
			if !ok {
				continue
			}
			name, ok := teamData["name"].(string)
			if !ok {
				continue
			}
			if !contains(teams, name) {
				teams = append(teams, name)
				wins[name] = 0
				loses[name] = 0
			}
		}

		team1, ok1 := opponentsRaw[0].(map[string]interface{})
		team2, ok2 := opponentsRaw[1].(map[string]interface{})
		if !ok1 || !ok2 {
			continue
		}
		team1Name, ok1 := team1["name"].(string)
		team2Name, ok2 := team2["name"].(string)
		if !ok1 || !ok2 {
			continue
		}

		winnerIndex, err := strconv.Atoi(winnerStr)
		if err != nil {
			continue // skip match if winner can't be parsed
		}

		switch winnerIndex {
		case 1:
			wins[team1Name]++
			loses[team2Name]++
		case 2:
			wins[team2Name]++
			loses[team1Name]++
		default:
			// Unexpected winner value, just skip
			continue
		}
	}

	scores := make(map[string]string)
	for _, team := range teams {
		scores[team] = fmt.Sprintf("%s: %d-%d", team, wins[team], loses[team])
	}

	return scores, nil
}