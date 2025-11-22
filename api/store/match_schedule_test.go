package store

import (
	"context"
	"os"
	"pickems-bot/api/external"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewTestStore(t *testing.T, round string) *Store {
	t.Helper()

	mongoURI := os.Getenv("MONGO_TEST_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://192.168.1.105:27017/?directConnection=true&serverSelectionTimeoutMS=2000"
	}

	clientOpts := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	db := client.Database("test")
	coll := db.Collection("scheduled_matches")
	_ = coll.Drop(context.TODO()) // clear before test

	s := &Store{
		Client:   client,
		Database: db,
		Round:    round,
	}

	s.Collections.MatchSchedule = coll
	return s
}

func TestStoreMatchSchedule_Update(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires MongoDB in CI environment")
	}

	store := NewTestStore(t, "test-round")

	original := []external.ScheduledMatch{
		{Team1: "TBD", Team2: "TBD", EpochTime: -62167219200, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
		{Team1: "TBD", Team2: "TBD", EpochTime: -62167219200, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
		{Team1: "FURIA", Team2: "PaiN Gaming", EpochTime: 1750359600, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
		{Team1: "Team Spirit", Team2: "MOUZ", EpochTime: 1750375800, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
		{Team1: "FaZe Clan", Team2: "The MongolZ", EpochTime: 1750442400, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
		{Team1: "Natus Vincere", Team2: "Team Vitality", EpochTime: 1750458600, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
		{Team1: "TBD", Team2: "TBD", EpochTime: 1750620600, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
	}

	if err := store.StoreMatchSchedule(original); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	updated := []external.ScheduledMatch{
		{Team1: "Team Spirit", Team2: "Team Vitality", EpochTime: -62167219200, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
		{Team1: "FaZe Clan", Team2: "FURIA", EpochTime: -62167219200, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
		{Team1: "FURIA", Team2: "PaiN Gaming", EpochTime: 1750359600, BestOf: "3", StreamURL: "BLAST_Premier", Finished: true},
		{Team1: "Team Spirit", Team2: "MOUZ", EpochTime: 1750375800, BestOf: "3", StreamURL: "BLAST_Premier", Finished: true},
		{Team1: "FaZe Clan", Team2: "The MongolZ", EpochTime: 1750442400, BestOf: "3", StreamURL: "BLAST_Premier", Finished: true},
		{Team1: "Natus Vincere", Team2: "Team Vitality", EpochTime: 1750458600, BestOf: "3", StreamURL: "BLAST_Premier", Finished: true},
		{Team1: "TBD", Team2: "TBD", EpochTime: 1750620600, BestOf: "3", StreamURL: "BLAST_Premier", Finished: false},
	}
	if err := store.StoreMatchSchedule(updated); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	var result UpcomingMatchDoc
	err := store.Collections.MatchSchedule.FindOne(context.TODO(), bson.M{"round": "test-round"}).Decode(&result)
	if err != nil {
		t.Fatalf("Fetch after update failed: %v", err)
	}
}
