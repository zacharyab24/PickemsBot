/* test_helpers.go
 * Contains test helper functions and mock structures for store package tests
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"pickems-bot/api/external"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewMockStore creates a Store instance for testing purposes.
// This can be used with a real test database or in-memory MongoDB.
func NewMockStore(dbName string, mongoURI string) (*Store, error) {
	return NewStore(dbName, mongoURI, "Test/Tournament/2025", "", "test_round")
}

// CreateTestStore creates a Store connected to a test database.
// Returns the store and a cleanup function.
func CreateTestStore(mongoURI string) (*Store, func(), error) {
	store, err := NewMockStore("test_pickems", mongoURI)
	if err != nil {
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
func CreateSampleScheduledMatches() []external.ScheduledMatch {
	return []external.ScheduledMatch{
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

// CreateSamplePrediction creates sample Prediction data for testing.
func CreateSamplePrediction(userID, username, format, round string) Prediction {
	if format == "swiss" {
		return Prediction{
			UserID:   userID,
			Username: username,
			Format:   format,
			Round:    round,
			Win:      []string{"Team A", "Team B"},
			Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
			Lose:     []string{"Team I", "Team J"},
		}
	}
	// single-elimination format
	return Prediction{
		UserID:   userID,
		Username: username,
		Format:   format,
		Round:    round,
	}
}
