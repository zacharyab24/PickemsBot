/* match_results.go
 * Contains the methods for interacting with the match_results collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"pickems-bot/api/external"
	"pickems-bot/api/format"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// FetchMatchResultsFromDb retrieves match results from the DB.
// It receives name of database as a string (e.g. user_pickems), receives name of collection as a string,
// (e.g. PW Shanghai Major 2024_results, and round as string (e.g. stage_1).
// It returns MatchResult interface if the operation was successful, or an error if it was not.
func (s *Store) FetchMatchResultsFromDb() (format.MatchResult, error) {
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

	f, err := format.Get(format.Kind(resultType))
	if err != nil {
		return nil, fmt.Errorf("unknown match result type: %s", resultType)
	}
	return f.DecodeBSON(bsonBytes)
}

// GetMatchResults returns the raw format.MatchResult from the DB. Format-aware
// conversion (record → MatchResult) is the caller's responsibility — it lives
// in the api/format package, which can't be imported here without a cycle.
func (s *Store) GetMatchResults() (format.MatchResult, error) {
	rec, err := s.FetchMatchResultsFromDb()
	if err != nil {
		return nil, fmt.Errorf("error occured getting match results from db: %w", err)
	}
	return rec, nil
}

// Helper function to get match data for a given liquipedia page. Note that the wiki is hard coded to counterstrike
// Preconditions: Receives string containing liquipedia page (such as BLAST/Major/2025/Austin/Stage_1) and optional params (such as  &section=24 (this is not used in majors))
// Postconditions: MatchResult interface containing either []MatchNode or map[string]string depending on the execution path, or error if it occurs
func (s *Store) fetchMatchDataFromExternal() (format.MatchResult, error) {
	url := fmt.Sprintf("https://liquipedia.net/counterstrike/%s?action=raw%s", s.Page, s.OptionalParams)

	// Get wikitext from url
	wikitext, err := external.GetWikitext(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching match2bracketid data: %w", err)
	}

	// Detect format from wikitext, then dispatch to the per-format ID extractor.
	kind, err := format.DetectKind(wikitext)
	if err != nil {
		return nil, fmt.Errorf("error detecting format: %w", err)
	}
	f, err := format.Get(kind)
	if err != nil {
		return nil, fmt.Errorf("unknown format type: %s", kind)
	}
	ids, _, err := f.ExtractMatchListIDs(wikitext)
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
	matchNodes, err := external.GetMatchNodesFromJSON(jsonResponse)
	if err != nil {
		return nil, fmt.Errorf("error parsing match data: %w", err)
	}

	return f.BuildFromMatchNodes(matchNodes, s.Round)
}

// StoreMatchResults persists a MatchResult to the DB. The record is BSON-marshalled
// using its struct tags and tagged with a top-level "type" discriminator so
// FetchMatchResultsFromDb can decode into the right concrete type later.
// Format-agnostic: works for any registered format without code changes here.
func (s *Store) StoreMatchResults(matchResult format.MatchResult) error {
	var raw bson.M
	err := s.Collections.MatchResults.FindOne(context.TODO(), bson.M{"round": s.Round}).Decode(&raw)
	notFound := err == mongo.ErrNoDocuments
	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing record failed: %w", err)
	}

	// Marshal the record with its BSON tags, then inject the type discriminator.
	doc := bson.M{"type": string(matchResult.GetType())}
	bsonBytes, err := bson.Marshal(matchResult)
	if err != nil {
		return fmt.Errorf("failed to marshal match result: %w", err)
	}
	var recordMap bson.M
	if err := bson.Unmarshal(bsonBytes, &recordMap); err != nil {
		return fmt.Errorf("failed to unmarshal match result into bson.M: %w", err)
	}
	for k, v := range recordMap {
		doc[k] = v
	}

	filter := bson.M{"round": s.Round}
	if notFound {
		if _, err := s.Collections.MatchResults.InsertOne(context.TODO(), doc); err != nil {
			return fmt.Errorf("failed to insert new match result: %w", err)
		}
		return nil
	}
	if _, err := s.Collections.MatchResults.UpdateOne(context.TODO(), filter, bson.M{"$set": doc}); err != nil {
		return fmt.Errorf("failed to update match result: %w", err)
	}
	return nil
}

// FetchAndUpdateMatchResults void function to fetch match results and update what is stored in the db
// Preconditions: DB has been initialised, store pointer is initialised
// Postconditions: Updates match results for configured tournament in DB
func (s *Store) FetchAndUpdateMatchResults() error {
	log.Println("updating match results stored in db...")
	// Get data from LiquipediaDB api
	externalResults, err := s.fetchMatchDataFromExternal()
	if err != nil {
		return err
	}

	// Validate liquipedia data — format-agnostic check via the unified interface
	if len(externalResults.GetTeamNames()) == 0 {
		return fmt.Errorf("no result returned from liquipediadb")
	}

	// Update match results in db
	if err := s.StoreMatchResults(externalResults); err != nil {
		return err
	}

	return nil
}
