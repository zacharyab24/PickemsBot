/* match_results.go
 * Contains the methods for interacting with the match_results collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"

	"pickems-bot/metrics"
	"pickems-bot/tournament"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// FetchMatchResultsFromDb retrieves the match result document for the current round and decodes it into the appropriate MatchResult implementation.
func (s *Store) FetchMatchResultsFromDb() (tournament.MatchResult, error) {
	metrics.MongoOpsTotal.WithLabelValues("read").Inc()
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

	f, err := tournament.Get(tournament.Kind(resultType))
	if err != nil {
		return nil, fmt.Errorf("unknown match result type: %s", resultType)
	}
	return f.DecodeBSON(bsonBytes)
}

// GetMatchResults returns the raw tournament.MatchResult from the DB. Format-aware
// conversion (record → MatchResult) is the caller's responsibility — it lives
// in the tournament package, which can't be imported here without a cycle.
func (s *Store) GetMatchResults() (tournament.MatchResult, error) {
	rec, err := s.FetchMatchResultsFromDb()
	if err != nil {
		return nil, fmt.Errorf("error occured getting match results from db: %w", err)
	}
	return rec, nil
}

// StoreMatchResults persists a MatchResult to the DB. The record is BSON-marshalled
// using its struct tags and tagged with a top-level "type" discriminator so
// FetchMatchResultsFromDb can decode into the right concrete type later.
// Format-agnostic: works for any registered format without code changes here.
func (s *Store) StoreMatchResults(matchResult tournament.MatchResult) error {
	metrics.MongoOpsTotal.WithLabelValues("write").Inc()
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

// FetchAndUpdateMatchResults fetches match data from the configured data source and stores the result in the db
func (s *Store) FetchAndUpdateMatchResults() error {
	result, nodes, err := s.Fetcher.FetchMatchData(s.Round)
	if err != nil {
		return err
	}
	if err := s.StoreMatchResults(result); err != nil {
		return err
	}
	if err := s.StoreMatchNodes(nodes, result.GetType()); err != nil {
		s.logger().Warn("failed to store match nodes", "error", fmt.Errorf("FetchAndUpdateMatchResults: %w", err))
	}
	return nil
}
