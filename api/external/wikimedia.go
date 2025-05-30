/* wikimedia.go
 * Contains the logic used to fetch data MediaWiki api and process the results
 * Authors: Zachary Bower
 */

package external

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Function to fetch raw wikitext from a given URL. This function does not perform any parsing on the text
// Preconditions: Receives string that contains URL for liquipedia page we wish to parse
// (e.g. https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Opening_Stage?action=raw)
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