package api

import (
	"fmt"
	"os"
	"pickems-bot/api/match"
	"sort"
)

// Function to get match data for a given liquipedia page. Note that the wiki is hard coded to counterstrike. This is the main run function of this file
// Preconditions: Receives string containing liquipedia page (such as BLAST/Major/2025/Austin/Stage_1) and optional params (such as  &section=24 (this is not used in majors))
// Postconditions: MatchResult interface containing either []MatchNode or map[string]string depending on the execution path, or error if it occurs
func GetMatchData(page string, optionalParams string) (match.MatchResult, error){
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
		fmt.Println("An error occured whilst fetching match data")
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
		rootNode, err := match.GetMatchTree(matchNodes)
		if err != nil {
			fmt.Println("An error occured whilst parsing match data: %w",err)
			return nil, fmt.Errorf("error creating match tree: %w", err)
		}
		return match.EliminationResult{TreeRoot: rootNode}, nil
		
	default:
		return nil, fmt.Errorf("unknown format type: %s", format)
	}
}

// Function to get data about upcoming matches. Returns a slice where each element contains: team1name, team2name, epoch time for match start, bestOf and twitch url
// Preconditions: Receives string containing liquipedia page (such as BLAST/Major/2025/Austin/Stage_1) and optional params (such as  &section=24 (this is not used in majors))
// Postconditions: Returns slice of UpcomingMatch, or error if it occurs
func GetUpcomingMatchData(page string, optionalParams string) ([]match.UpcomingMatch, error){
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
		fmt.Println("An error occured whilst fetching match data")
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