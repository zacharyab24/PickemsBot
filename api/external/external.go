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
)

// GetWikitext fetches raw wikitext from a given URL. This function does not perform any parsing on the text.
// It receives string that contains URL for liquipedia page we wish to parse
// (e.g. https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Opening_Stage?action=raw).
// It returns string containing raw wiki text and errors.
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
