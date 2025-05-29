/* api.go
 * This file contains the high level logic for interacting with this package. For consistent results, fuctions should
 * only be called from this file, not the sub packages for match and processing. For detauls about functionality see `api.md`
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */
package api

import (
	"errors"
	"fmt"
	"os"
	match "pickems-bot/api/match_data"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

// Function to get match data for a given liquipedia page. Note that the wiki is hard coded to counterstrike. This is the main run function of this file
// Preconditions: Receives string containing liquipedia page (such as BLAST/Major/2025/Austin/Stage_1) and optional params (such as  &section=24 (this is not used in majors))
// Postconditions: MatchResult interface containing either []MatchNode or map[string]string depending on the execution path, or error if it occurs
func fetchMatchData(page string, optionalParams string) (match.MatchResult, error){
	url := fmt.Sprintf("https://liquipedia.net/counterstrike/%s?action=raw%s", page, optionalParams)
	
	// Get wikitext from url
	wikitext, err := match.GetWikitext(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching match2bracketid data: %w", err)
	}

	// Get match2bracketid's from wikitext
	ids, format, err := match.ExtractMatchListId(wikitext)
	if err != nil {
		return nil, fmt.Errorf("error extracting match list: %w", err)
	}

	// Get JSON match data filtered by match2bracketid
	liquipediaDBApiKey := os.Getenv("LIQUIDPEDIADB_API_KEY")
	jsonResponse, err := match.GetLiquipediaMatchData(liquipediaDBApiKey, ids)
	if err != nil {
		return nil, fmt.Errorf("error fetching match data from liquipedia api: %w", err)
	}

	// Get match nodes from jsonResponse
	matchNodes, err := match.GetMatchNodesFromJson(jsonResponse)
	if err != nil {
		return nil, fmt.Errorf("error parsing match data: %w", err)
	}

	// Get return values depending on tournament type
	switch format {
	case "swiss":
		scores, err := match.CalculateSwissScores(matchNodes)
		if err != nil {
			return nil, fmt.Errorf("error calculating swiss scores: %w", err)
		}
		return match.SwissResult{Scores: scores}, nil

	case "single-elimination":
		progression, err := match.GetEliminationResults(matchNodes)
		if err != nil {
			return nil, fmt.Errorf("error creating match tree: %w", err)
		}
		return match.EliminationResult{Progression: progression}, nil
		
	default:
		return nil, fmt.Errorf("unknown format type: %s", format)
	}
}

// Function to get data about upcoming matches. Returns a slice where each element contains: team1name, team2name, epoch time for match start, bestOf and twitch url
// Preconditions: Receives string containing liquipedia page (such as BLAST/Major/2025/Austin/Stage_1) and optional params (such as  &section=24 (this is not used in majors))
// Postconditions: Returns slice of UpcomingMatch, or error if it occurs
func FetchUpcomingMatches(page string, optionalParams string) ([]match.UpcomingMatch, error){
	url := fmt.Sprintf("https://liquipedia.net/counterstrike/%s?action=raw%s", page, optionalParams)
	
	// Get wikitext from url
	wikitext, err := match.GetWikitext(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching match2bracketid data: %w", err)
	}

	// Get match2bracketid's from wikitext
	ids, _, err := match.ExtractMatchListId(wikitext)
	if err != nil {
		return nil, fmt.Errorf("error extracting match list: %w", err)
	}

	// Get JSON match data filtered by match2bracketid
	liquipediaDBApiKey := os.Getenv("LIQUIDPEDIADB_API_KEY")
	jsonResponse, err := match.GetLiquipediaMatchData(liquipediaDBApiKey, ids)
	if err != nil {
		return nil, fmt.Errorf("error fetching match data from liquipedia api: %w", err)
	}

	// Get upcoming matches (if any) from jsonResponse
	upcomingMatches, err := match.GetUpcomingMatchesFromJson(jsonResponse)
	if err != nil {
		return nil, err
	}

	// Sort slices by epoch time
	sort.Slice(upcomingMatches, func(i, j int) bool {
		return upcomingMatches[i].EpochTime < upcomingMatches[j].EpochTime
	})

	return upcomingMatches, nil
}

// Function to get match results. Checks if the data in the db is outdated, if it is, makes api call to liquipediaDb api and updates local db
// Precondtions: recieves string containing dbName, colName, round, page and params (all of these come from flags at start up)
// Postconditions: Returns MatchResult containing the lastest match data, or an error if it occurs
func GetMatchResults(dbName string, collName string, round string, page string, params string) (match.MatchResult, error) {
	// Get results stored in our db
	dbResults, err := match.FetchMatchResultsFromDb(dbName, collName, round)
	var shouldRefresh bool
	if err != nil {
		// If this is triggered, there are no match results currently saved in the db
		if errors.Is(err, mongo.ErrNoDocuments) {
			shouldRefresh = true
		} else {
			return nil, fmt.Errorf("error occured getting match results from db: %w", err)
		}
	} else if dbResults.GetTTL() < time.Now().Unix() {
		shouldRefresh = true
	}
	
	// Run if we need to refresh the data stored in the db (either there is no data stored or the TTL has experied)
	if shouldRefresh {
		fmt.Println("updating match results stored in db...")
		// Get data from LiquipediaDB api
		externalResults, err := fetchMatchData(page, params)
		if err != nil {
			return nil, err
		}
		
		// Validate liquipedia data
		switch externalResults.GetType() {
		case "swiss":
			swissResult, ok := externalResults.(match.SwissResult)
			if !ok {
				return nil, fmt.Errorf("could not assert MatchResult to SwissResult")
			}
			if len(swissResult.Scores) == 0 {
				return nil, fmt.Errorf("no result returned from liquipediadb")
			}
		case "single-elimination":
			elimResult, ok := externalResults.(match.EliminationResult)
			if !ok {
				return nil, fmt.Errorf("could not assert MatchResult to EliminationResult")
			}
			if len(elimResult.Progression) == 0 {
				return nil, fmt.Errorf("no result returned from liquipediadb")
			}
		default:
			return nil, fmt.Errorf("unknown format type returned from liquipediadb")
		}

		// Get upcoming matches from db
		upcomingMatches, err := match.FetchUpcomingMatchesFromDb(dbName, "upcoming_matches", round)
		if err != nil {
			return nil, err
		}

		// Update match results in db
		err = match.StoreMatchResults(dbName, collName, externalResults, round, upcomingMatches)
		if err != nil {
			return nil, err
		}

		return externalResults, nil
	}

	// Else we can return the cached data

	matchResult, err := match.ToMatchResult(dbResults) 
	if err != nil {
		return nil, err
	}
	return matchResult, nil
}

// Helper function to get valid team names used in setting user predictions. We are going to grab the valid team names
// from the results table as this already contains a list of names, and lets us filter by round without needing to
// create and maintain a new collection that will require more api calls
// Preconditions: Receives db name, collection name and round strings
// Postconditions: Returns string slice containing valid team names for the round, or returns error if an issue occurs 
func GetValidTeams(dbName string, collName string, round string) ([]string, string, error) {
	// Get results stored in our db
	dbResults, err := match.FetchMatchResultsFromDb(dbName, collName, round)
	if err != nil {
		return nil, "", err
	}

 	var teamNames []string

    // Type assertion to determine the concrete type and extract team names
    switch result := dbResults.(type) {
    case match.SwissResultRecord:
        // For Swiss format, Teams is map[string]string
        for teamName := range result.Teams {
			teamNames = append(teamNames, teamName)
        }
    case match.EliminationResultRecord:
        // For Elimination format, Progression is map[string]*TeamProgress
        for teamName := range result.Teams {
            teamNames = append(teamNames, teamName)
        }
    default:
        return nil, "", fmt.Errorf("unknown result record type: %T", result)
    }

    return teamNames,dbResults.GetType(), nil
}