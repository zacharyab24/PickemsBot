/* external.go
 * Contains the logic used to fetch data from external MediaWiki and LiquipediaDB apis, and return the results to the
 * higher level functions
 * Authors: Zachary Bower
 */

package external

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

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
