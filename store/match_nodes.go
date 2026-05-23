/* match_nodes.go
 * Contains the methods for interacting with the match_nodes collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"

	"pickems-bot/sources"
	"pickems-bot/tournament"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// StoreMatchNodes persists the raw []MatchNode slice for a round so it can be
// used later for results display and bracket rendering.
func (s *Store) StoreMatchNodes(nodes []sources.MatchNode, kind tournament.Kind) error {
	filter := bson.M{"round": s.Round}

	doc := bson.M{
		"round":  s.Round,
		"format": string(kind),
		"nodes":  nodes,
	}

	var existing bson.M
	err := s.Collections.MatchNodes.FindOne(context.TODO(), filter).Decode(&existing)
	notFound := errors.Is(err, mongo.ErrNoDocuments)
	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing match nodes record failed: %w", err)
	}

	if notFound {
		if _, err := s.Collections.MatchNodes.InsertOne(context.TODO(), doc); err != nil {
			return fmt.Errorf("failed to insert match nodes: %w", err)
		}
		return nil
	}
	if _, err := s.Collections.MatchNodes.UpdateOne(context.TODO(), filter, bson.M{"$set": doc}); err != nil {
		return fmt.Errorf("failed to update match nodes: %w", err)
	}
	return nil
}

// FetchMatchNodesFromDb retrieves the raw []MatchNode slice for the configured round, and the tournament.Kind of the round
// tournament.Kind could potentially be an empty string if legacy data is fetched, so callers should check that
func (s *Store) FetchMatchNodesFromDb() ([]sources.MatchNode, tournament.Kind, error) {
	var doc struct {
		Nodes  []sources.MatchNode `bson:"nodes"`
		Format tournament.Kind     `bson:"format"`
	}
	err := s.Collections.MatchNodes.FindOne(context.TODO(), bson.M{"round": s.Round}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("error fetching match nodes from db: %w", err)
	}
	return doc.Nodes, doc.Format, nil
}
