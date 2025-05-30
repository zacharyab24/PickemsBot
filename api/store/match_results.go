/* match_results.go
 * Contains the methods for interacting with the match_results collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"pickems-bot/api/external"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Function to retrieve match results from the DB
// Preconditions: Receives name of database as a string (e.g. user_pickems), receives name of collection as a string,
// (e.g. PW Shanghai Major 2024_results, and round as string (e.g. stage_1)
// Postconditions: Returns MatchResult interface if the operation was successful, or an error if it was not
func (s *Store) FetchMatchResultsFromDb() (ResultRecord, error) {
	s.Collections.MatchResults.Name()
	opts := options.FindOne()
	
	// MatchResult is an interface, which can't be decoded by MongoDB's driver. Instead need to get raw and convert to interface later
	var raw bson.M

	err := s.Collections.MatchResults.FindOne(context.TODO(), bson.D{{Key: "round", Value: s.Round}}, opts).Decode(&raw)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		return nil, fmt.Errorf("error fetching results from db: %w", err)
	}

	// Determine which type of MatchResult we fetched
	resultType, ok := raw["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid `type` field in document")
	}

	bsonBytes, err := bson.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw bson: %w", err)
	}

	// Decode results and map to correct struct for format
	switch resultType {
	case "swiss":
		var swiss SwissResultRecord
		if err := bson.Unmarshal(bsonBytes, &swiss); err != nil {
			return nil, fmt.Errorf("failed to decode SwissResultRecord: %w", err)
		}
		return swiss, nil
	case "single-elimination":
		var elim EliminationResultRecord
		if err := bson.Unmarshal(bsonBytes, &elim); err != nil {
			return nil, fmt.Errorf("failed to decode EliminationResultRecord: %w", err)
		}
		return elim, nil
	default:
		return nil, fmt.Errorf("unknown match result type: %s", resultType)
	}
}

// Function to get match results. Checks if the data in the db is outdated, if it is, makes api call to liquipediaDb api and updates local db
// Precondtions: recieves string containing dbName, colName, round, page and params (all of these come from flags at start up)
// Postconditions: Returns MatchResult containing the lastest match data, or an error if it occurs
func (s *Store) GetMatchResults() (external.MatchResult, error) {
	// Get results stored in our db
	dbResults, err := s.FetchMatchResultsFromDb()
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
		externalResults, err := s.fetchMatchDataFromExternal()
		if err != nil {
			return nil, err
		}
		
		// Validate liquipedia data
		switch externalResults.GetType() {
		case "swiss":
			swissResult, ok := externalResults.(external.SwissResult)
			if !ok {
				return nil, fmt.Errorf("could not assert MatchResult to SwissResult")
			}
			if len(swissResult.Scores) == 0 {
				return nil, fmt.Errorf("no result returned from liquipediadb")
			}
		case "single-elimination":
			elimResult, ok := externalResults.(external.EliminationResult)
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
		upcomingMatches, err := s.FetchUpcomingMatchesFromDb()
		if err != nil {
			return nil, err
		}

		// Update match results in db
		err = s.StoreMatchResults(externalResults, upcomingMatches)
		if err != nil {
			return nil, err
		}

		return externalResults, nil
	}

	// Else we can return the cached data

	matchResult, err := ToMatchResult(dbResults) 
	if err != nil {
		return nil, err
	}
	return matchResult, nil
}

// Helper function to get match data for a given liquipedia page. Note that the wiki is hard coded to counterstrike
// Preconditions: Receives string containing liquipedia page (such as BLAST/Major/2025/Austin/Stage_1) and optional params (such as  &section=24 (this is not used in majors))
// Postconditions: MatchResult interface containing either []MatchNode or map[string]string depending on the execution path, or error if it occurs
func (s *Store) fetchMatchDataFromExternal() (external.MatchResult, error){
	url := fmt.Sprintf("https://liquipedia.net/counterstrike/%s?action=raw%s", s.Page, s.OptionalParams)
	
	// Get wikitext from url
	wikitext, err := external.GetWikitext(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching match2bracketid data: %w", err)
	}

	// Get match2bracketid's from wikitext
	ids, format, err := external.ExtractMatchListId(wikitext)
	if err != nil {
		return nil, fmt.Errorf("error extracting match list: %w", err)
	}

	// Get JSON match data filtered by match2bracketid
	liquipediaDBApiKey := os.Getenv("LIQUIDPEDIADB_API_KEY")
	jsonResponse, err := external.GetLiquipediaMatchData(liquipediaDBApiKey, ids)
	if err != nil {
		return nil, fmt.Errorf("error fetching match data from liquipedia api: %w", err)
	}

	// Get match nodes from jsonResponse
	matchNodes, err := external.GetMatchNodesFromJson(jsonResponse)
	if err != nil {
		return nil, fmt.Errorf("error parsing match data: %w", err)
	}

	// Get return values depending on tournament type
	switch format {
	case "swiss":
		scores, err := external.CalculateSwissScores(matchNodes)
		if err != nil {
			return nil, fmt.Errorf("error calculating swiss scores: %w", err)
		}
		return external.SwissResult{Scores: scores}, nil

	case "single-elimination":
		progression, err := external.GetEliminationResults(matchNodes)
		if err != nil {
			return nil, fmt.Errorf("error creating match tree: %w", err)
		}
		return external.EliminationResult{Progression: progression}, nil
		
	default:
		return nil, fmt.Errorf("unknown format type: %s", format)
	}
}

// Function used to store match results in the db
// Preconditions: Receives name of database as a string (e.g. user_pickems), receives name of collection as a string, (e.g.)
// PW Shanghai Major 2024_results, MatchResult inferface containing the data to be stored, and round as a string (e.g. stage_1)
// Postconditions: Updates the data stored in the db, returns error message if the operation was unsuccessful
func (s *Store) StoreMatchResults(matchResult external.MatchResult, upcomingMatches []external.UpcomingMatch) error {
	
	// Attempt to find an existing document
	var raw bson.M
	err := s.Collections.MatchResults.FindOne(context.TODO(), bson.M{"round": s.Round}).Decode(&raw)
	notFound := err == mongo.ErrNoDocuments

	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing record failed: %w", err)
	}

	ttl := DetermineTTL(upcomingMatches)

	var update bson.M
	var newRecord interface{}
	var filter = bson.M{"round": s.Round}

	switch typed := matchResult.(type) {
	case external.SwissResult:
		update = bson.M{
			"$set": bson.M{
				"ttl":   ttl,
				"teams": typed.Scores,
			},
		}
		newRecord = bson.M{
			"type": typed.GetType(),
			"round": s.Round,
			"ttl": ttl,
			"teams": typed.Scores,
		}

	case external.EliminationResult:
		update = bson.M{
			"$set": bson.M{
				"ttl":   ttl,
				"teams": typed.Progression,
			},
		}
		newRecord = bson.M{
			"type": typed.GetType(),
			"round": s.Round,
			"ttl": ttl,
			"teams": typed.Progression,
		}

	default:
		return fmt.Errorf("unknown match result type: %s", matchResult.GetType())
	}

	// Perform insert or update
	if notFound {
		_, err := s.Collections.MatchResults.InsertOne(context.TODO(), newRecord)
		if err != nil {
			return fmt.Errorf("failed to insert new match result: %w", err)
		}
		return nil
	}

	_, err = s.Collections.MatchResults.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update match result: %w", err)
	}
	return nil
}

// Function for calculating TTL for result caching. If there are ongoing matches this is shortTTL, else normalTTL. These
// values are defined in a const within the function
// Preconditions: Receives slice of UpcomingMatch which contains information about the match
// Postconditions: Returns time.Duration with the value for the TTL
func DetermineTTL(matches []external.UpcomingMatch) int64 {
	now := time.Now().Unix()

	const (
		shortTTL = 3 * time.Minute // When there is a match ongoing TTL is 3 minutes
		normalTTL = 30 * time.Minute // Else it is 30
	)

	for _, match := range matches {
		var estimatedMatchDuration int64
		switch strings.ToLower(match.BestOf) {
		case "1": //BO1 time estimate is 90 mins
			estimatedMatchDuration = 1 * 60 * 90
		case "3": //BO3 time estimate is 4 hours
			estimatedMatchDuration = 4 * 60 * 60
		case "5": // BO5 time estimate is 6 hours
			estimatedMatchDuration = 6 * 60 * 60
		default: // If there is different or missing BestOf value default to 3 hours
			estimatedMatchDuration = 3 * 60 * 60
		}

		startTime := match.EpochTime
		finishedTime := startTime + estimatedMatchDuration

		// We are defining an ongoing match to be a match between the start time and estimated finish time
		// It would be possible to check if the match has finished from the Liquipedia API but that adds an extra request
		// to the rate limiter
		if now >= startTime && now <= finishedTime {
			return time.Now().Add(shortTTL).Unix()
		}
	}
	// No ongoing match
	return time.Now().Add(normalTTL).Unix()
}
