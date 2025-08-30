/* external.go
 * Contains the logic used to fetch data from external MediaWiki and LiquipediaDB apis, and return the results to the
 * higher level functions
 * Authors: Zachary Bower
 */

package external

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Function to get scheduled matches, and return the results
// Preconditions: Receives string containing api key
// Returns slice of Scheduled matches or an error if it occurs
func FetchScheduledMatches(apiKey string, page string, optionalParams string) ([]ScheduledMatch, error) {
	url := fmt.Sprintf("https://liquipedia.net/counterstrike/%s?action=raw%s", page, optionalParams)

	// Get wikitext
	wikitext, err := GetWikitext(url)
	if err != nil {
		return nil, fmt.Errorf("error getting wikitext: %w", err)
	}

	// Get match2bracketid's from wikitext
	ids, _, err := ExtractMatchListId(wikitext)
	if err != nil {
		return nil, fmt.Errorf("error extracting match list: %w", err)
	}

	// Get match data from liquipedia db
	jsonResponse, err := GetLiquipediaMatchData(apiKey, ids)
	if err != nil {
		return nil, fmt.Errorf("error fetching match data from liquipedia api: %w", err)
	}

	// Get scheduled matches (if any) from jsonResponse
	scheduledMatches, err := GetScheduledMatchesFromJson(jsonResponse)
	if err != nil {
		return nil, err
	}

	// Sort slices by epoch time
	sort.Slice(scheduledMatches, func(i, j int) bool {
		return scheduledMatches[i].EpochTime < scheduledMatches[j].EpochTime
	})

	return scheduledMatches, nil
}

// Function to fetch raw wikitext from a given URL. This function does not perform any parsing on the text
// Preconditions: Receives string that contains URL for liquipedia page we wish to parse
// (e.g. https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Opening_Stage?action=raw)
// Postconditions: Returns string containing raw wiki text and errors
func GetWikitext(url string) (string, error) {

	// Create HTTP Request
	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
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
		fmt.Printf("Failed to fetch page. Status code: %d\n", response.StatusCode)
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
		fmt.Println("Failed to read response body:", err)
		return "", err
	}

	return string(body), err
}

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
		fmt.Println("Invalid url:", err)
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
	request, err := http.NewRequest("GET", parsedUrl.String(), nil)
	if err != nil {
		fmt.Println("Failed to create request", err)
		return "", err
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
