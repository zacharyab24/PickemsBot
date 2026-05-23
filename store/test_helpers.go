/* test_helpers.go
 * Contains test helper functions and mock structures for store package tests
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"time"

	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/tournament"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewMockStore creates a Store instance for testing purposes.
// This can be used with a real test database or in-memory MongoDB.
func NewMockStore(dbName string, mongoURI string) (*Store, error) {
	return NewStore(dbName, mongoURI, "test_round", NewLiquipediaFetcher("", "Test/Tournament/2025"))
}

// CreateTestStore creates a Store connected to a test database.
// Returns the store and a cleanup function.
func CreateTestStore(mongoURI string) (*Store, func(), error) {
	store, err := NewMockStore("test_pickems", mongoURI)
	if err != nil {
		return nil, nil, err
	}

	// Verify the connection is actually reachable before proceeding.
	// mongo.Connect is lazy — it doesn't dial until the first operation.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Client.Ping(ctx, nil); err != nil {
		store.Client.Disconnect(context.TODO())
		return nil, nil, err
	}

	cleanup := func() {
		if store.Client != nil {
			// Drop test database
			store.Database.Drop(context.TODO())
			// Disconnect client
			store.Client.Disconnect(context.TODO())
		}
	}

	return store, cleanup, nil
}

// CreateTestClient creates a test MongoDB client.
func CreateTestClient(mongoURI string) (*mongo.Client, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}
	return client, nil
}

// CreateSampleScheduledMatches creates sample ScheduledMatch data for testing.
func CreateSampleScheduledMatches() []sources.ScheduledMatch {
	return []sources.ScheduledMatch{
		{
			Team1:     "Team A",
			Team2:     "Team B",
			BestOf:    "3",
			EpochTime: 1700000000,
			StreamURL: "BLAST",
			Finished:  false,
		},
		{
			Team1:     "Team C",
			Team2:     "Team D",
			BestOf:    "3",
			EpochTime: 1700010000,
			StreamURL: "BLAST_Premier",
			Finished:  false,
		},
	}
}

// CreateSamplePrediction creates sample models.Prediction data for testing.
func CreateSamplePrediction(userID, username string, kind tournament.Kind, round string) models.Prediction {
	if kind == tournament.Swiss {
		return models.Prediction{
			UserID:   userID,
			Username: username,
			Format:   string(kind),
			Round:    round,
			Win:      []string{"Team A", "Team B"},
			Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
			Lose:     []string{"Team I", "Team J"},
		}
	}
	// single-elimination format
	return models.Prediction{
		UserID:   userID,
		Username: username,
		Format:   string(kind),
		Round:    round,
	}
}
