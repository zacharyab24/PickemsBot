/* match_schedule_test.go
 * Contains unit tests for match_schedule.go
 * Authors: Zachary Bower
 * AI-Generated: Additional tests for FetchMatchSchedule, EnsureScheduledMatches
 */

package store

import (
	"context"
	"os"
	"pickems-bot/api/external"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
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

// region FetchMatchSchedule tests (using mtest mocks)

func TestFetchMatchSchedule_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully fetches match schedule", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning scheduled matches
		scheduleDoc := mtest.CreateCursorResponse(1, "test.scheduled_matches", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
			{Key: "ttl", Value: int64(1700000000)},
			{Key: "scheduled_matches", Value: bson.A{
				bson.D{
					{Key: "team1", Value: "Team A"},
					{Key: "team2", Value: "Team B"},
					{Key: "epochtime", Value: int64(1700000000)},
					{Key: "bestof", Value: "3"},
					{Key: "streamurl", Value: "BLAST_Premier"},
					{Key: "finished", Value: false},
				},
				bson.D{
					{Key: "team1", Value: "Team C"},
					{Key: "team2", Value: "Team D"},
					{Key: "epochtime", Value: int64(1700010000)},
					{Key: "bestof", Value: "3"},
					{Key: "streamurl", Value: "BLAST_Premier"},
					{Key: "finished", Value: false},
				},
			}},
		})
		mt.AddMockResponses(scheduleDoc)

		matches, err := store.FetchMatchSchedule()
		require.NoError(t, err)
		require.NotNil(t, matches)
		assert.Len(t, matches, 2)
		assert.Equal(t, "Team A", matches[0].Team1)
		assert.Equal(t, "Team B", matches[0].Team2)
		assert.Equal(t, "3", matches[0].BestOf)
	})
}

func TestFetchMatchSchedule_DatabaseError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error on database failure", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		matches, err := store.FetchMatchSchedule()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error fetching results from db")
		assert.Nil(t, matches)
	})
}

func TestFetchMatchSchedule_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when no schedule found", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.scheduled_matches", mtest.FirstBatch))

		matches, err := store.FetchMatchSchedule()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error fetching results from db")
		assert.Nil(t, matches)
	})
}

// endregion

// region StoreMatchSchedule tests (using mtest mocks)

func TestStoreMatchSchedule_InsertNew(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully inserts new match schedule", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning no documents (new schedule)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.scheduled_matches", mtest.FirstBatch))
		// Mock InsertOne success
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		matches := CreateSampleScheduledMatches()

		err := store.StoreMatchSchedule(matches)
		assert.NoError(t, err)
	})
}

func TestStoreMatchSchedule_UpdateExisting(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully updates existing match schedule", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning existing document - need cursor response with getMore
		first := mtest.CreateCursorResponse(1, "test.scheduled_matches", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
		})
		getMore := mtest.CreateCursorResponse(0, "test.scheduled_matches", mtest.NextBatch)
		// Mock UpdateOne success
		updateSuccess := bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
		}
		mt.AddMockResponses(first, getMore, updateSuccess)

		matches := CreateSampleScheduledMatches()

		err := store.StoreMatchSchedule(matches)
		assert.NoError(t, err)
	})
}

func TestStoreMatchSchedule_EmptySlice(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when empty slice provided", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		err := store.StoreMatchSchedule([]external.ScheduledMatch{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "scheduled matches input has length 0")
	})
}

func TestStoreMatchSchedule_FindOneError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when FindOne fails", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		matches := CreateSampleScheduledMatches()

		err := store.StoreMatchSchedule(matches)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lookup for existing record failed")
	})
}

func TestStoreMatchSchedule_InsertError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when insert fails", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.scheduled_matches", mtest.FirstBatch))
		// Mock InsertOne error
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "insert failed",
		}))

		matches := CreateSampleScheduledMatches()

		err := store.StoreMatchSchedule(matches)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert upcoming matches")
	})
}

func TestStoreMatchSchedule_UpdateError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when update fails", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning existing document
		existingDoc := mtest.CreateCursorResponse(1, "test.scheduled_matches", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
		})
		mt.AddMockResponses(existingDoc)
		// Mock UpdateOne error
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "update failed",
		}))

		matches := CreateSampleScheduledMatches()

		err := store.StoreMatchSchedule(matches)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update upcoming matches")
	})
}

// endregion

// region EnsureScheduledMatches tests

func TestEnsureScheduledMatches_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns nil when scheduled matches exist", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning document with scheduled matches
		scheduleDoc := mtest.CreateCursorResponse(1, "test.scheduled_matches", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
			{Key: "scheduled_matches", Value: bson.A{
				bson.D{
					{Key: "team1", Value: "Team A"},
					{Key: "team2", Value: "Team B"},
				},
			}},
		})
		mt.AddMockResponses(scheduleDoc)

		err := store.EnsureScheduledMatches()
		assert.NoError(t, err)
	})
}

func TestEnsureScheduledMatches_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when no document found", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.scheduled_matches", mtest.FirstBatch))

		err := store.EnsureScheduledMatches()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no scheduled matches entry found for round test_round")
	})
}

func TestEnsureScheduledMatches_EmptyMatches(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when scheduled matches are empty", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning document with empty scheduled_matches
		scheduleDoc := mtest.CreateCursorResponse(1, "test.scheduled_matches", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
			{Key: "scheduled_matches", Value: bson.A{}},
		})
		mt.AddMockResponses(scheduleDoc)

		err := store.EnsureScheduledMatches()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "scheduled match collection found but its results were empty")
	})
}

func TestEnsureScheduledMatches_DatabaseError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error on database failure", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchSchedule: mt.Coll,
			},
		}

		// Mock FindOne returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		err := store.EnsureScheduledMatches()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error checking scheduled matches")
	})
}

// endregion
