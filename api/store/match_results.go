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

// updateMatchResultsFromJSON parses match nodes from a LiquipediaDB JSON response,
// detects the tournament format, builds the MatchResult, and persists both the
// result and the raw nodes. This is the shared core used by both fetch paths.
func (s *Store) updateMatchResultsFromJSON(jsonResponse string) error {
	matchNodes, err := external.GetMatchNodesFromJSON(jsonResponse)
	if err != nil {
		return fmt.Errorf("error parsing match data: %w", err)
	}

	// Use the config-level format override when set; otherwise auto-detect from
	// the section keywords present in the match nodes. The override is needed
	// when a single Liquipedia page contains multiple stages (e.g. Swiss group
	// stage AND a single-elimination playoffs bracket) and auto-detection would
	// resolve to the wrong format.
	var kind format.Kind
	if s.Format != "" {
		kind = format.Kind(s.Format)
		if _, err := format.Get(kind); err != nil {
			return fmt.Errorf("invalid format override %q in config: %w", s.Format, err)
		}
	} else {
		kind, err = format.DetectKindFromMatchNodes(matchNodes)
		if err != nil {
			return fmt.Errorf("error detecting format from match nodes: %w", err)
		}
	}

	// Filter out matches that don't belong to the detected/configured format
	// (e.g. remove playoffs and showmatch sections when targeting Swiss rounds,
	// or remove Swiss rounds when targeting a playoffs bracket).
	matchNodes = format.FilterNodesByKind(matchNodes, kind)

	f, err := format.Get(kind)
	if err != nil {
		return fmt.Errorf("unknown format type: %s", kind)
	}

	result, err := f.BuildFromMatchNodes(matchNodes, s.Round)
	if err != nil {
		return err
	}

	if len(result.GetTeamNames()) == 0 {
		return fmt.Errorf("no result returned from liquipediadb")
	}

	if err := s.StoreMatchResults(result); err != nil {
		return err
	}

	if err := s.StoreMatchNodes(matchNodes, result.GetType()); err != nil {
		log.Printf("warning: failed to store match nodes: %v", err)
	}

	return nil
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

// FetchAndUpdateMatchResults fetches match data from LiquipediaDB and persists
// the results. Used by the webhook-triggered update path (UpdateMatchResults).
// Preconditions: DB has been initialised, LIQUIDPEDIADB_API_KEY env var set
// Postconditions: Updates match results for configured tournament in DB
func (s *Store) FetchAndUpdateMatchResults() error {
	log.Println("updating match results stored in db...")
	jsonResponse, err := external.GetLiquipediaMatchDataByPage(os.Getenv("LIQUIDPEDIADB_API_KEY"), s.Page)
	if err != nil {
		return fmt.Errorf("error fetching match data from liquipedia api: %w", err)
	}
	return s.updateMatchResultsFromJSON(jsonResponse)
}

// FetchAndUpdateMatchResultsFromJSON persists match results from a pre-fetched
// LiquipediaDB JSON response. Used by PopulateMatches to avoid a duplicate API
// call when the same JSON was already fetched for the schedule.
// Preconditions: jsonResponse is a valid LiquipediaDB /match response
// Postconditions: Updates match results for configured tournament in DB
func (s *Store) FetchAndUpdateMatchResultsFromJSON(jsonResponse string) error {
	log.Println("updating match results stored in db...")
	return s.updateMatchResultsFromJSON(jsonResponse)
}
