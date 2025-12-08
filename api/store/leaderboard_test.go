/* leaderboard_test.go
 * Contains unit tests for leaderboard.go
 * AI-Generated
 */

package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// region FetchLeaderboardFromDB tests

func TestFetchLeaderboardFromDB_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully fetches leaderboard", func(mt *mtest.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning leaderboard
		leaderboardDoc := mtest.CreateCursorResponse(1, "test.leaderboard", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
			{Key: "updated_at", Value: time.Now()},
			{Key: "entries", Value: bson.A{
				bson.D{
					{Key: "userid", Value: "user1"},
					{Key: "username", Value: "TestUser1"},
					{Key: "score", Value: 10},
					{Key: "successes", Value: 5},
					{Key: "pending", Value: 3},
					{Key: "failed", Value: 2},
				},
				bson.D{
					{Key: "userid", Value: "user2"},
					{Key: "username", Value: "TestUser2"},
					{Key: "score", Value: 8},
					{Key: "successes", Value: 4},
					{Key: "pending", Value: 2},
					{Key: "failed", Value: 4},
				},
			}},
		})
		mt.AddMockResponses(leaderboardDoc)

		entries, err := store.FetchLeaderboardFromDB()
		require.NoError(t, err)
		require.NotNil(t, entries)
		assert.Len(t, entries, 2)
		assert.Equal(t, "user1", entries[0].UserID)
		assert.Equal(t, "TestUser1", entries[0].Username)
		assert.Equal(t, 10, entries[0].Score)
		assert.Equal(t, 5, entries[0].Successes)
		assert.Equal(t, "user2", entries[1].UserID)
	})
}

func TestFetchLeaderboardFromDB_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when no leaderboard found", func(mt *mtest.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.leaderboard", mtest.FirstBatch))

		entries, err := store.FetchLeaderboardFromDB()
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Nil(t, entries)
	})
}

func TestFetchLeaderboardFromDB_DatabaseError(t *testing.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		entries, err := store.FetchLeaderboardFromDB()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch leaderboard from database")
		assert.Nil(t, entries)
	})
}

func TestFetchLeaderboardFromDB_EmptyEntries(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns empty slice when leaderboard has no entries", func(mt *mtest.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning leaderboard with empty entries
		leaderboardDoc := mtest.CreateCursorResponse(1, "test.leaderboard", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
			{Key: "updated_at", Value: time.Now()},
			{Key: "entries", Value: bson.A{}},
		})
		mt.AddMockResponses(leaderboardDoc)

		entries, err := store.FetchLeaderboardFromDB()
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

// endregion

// region StoreLeaderboard tests

func TestStoreLeaderboard_InsertNew(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully inserts new leaderboard", func(mt *mtest.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning no documents (new leaderboard)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.leaderboard", mtest.FirstBatch))
		// Mock InsertOne success
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		leaderboard := Leaderboard{
			Round:     "test_round",
			UpdatedAt: time.Now(),
			Entries: []LeaderboardEntry{
				{
					UserID:   "user1",
					Username: "TestUser1",
					Score:    10,
					ScoreResult: ScoreResult{
						Successes: 5,
						Pending:   3,
						Failed:    2,
					},
				},
			},
		}

		err := store.StoreLeaderboard(leaderboard)
		assert.NoError(t, err)
	})
}

func TestStoreLeaderboard_UpdateExisting(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully updates existing leaderboard", func(mt *mtest.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning existing document - need cursor response with getMore
		first := mtest.CreateCursorResponse(1, "test.leaderboard", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
		})
		getMore := mtest.CreateCursorResponse(0, "test.leaderboard", mtest.NextBatch)
		// Mock UpdateOne success
		updateSuccess := bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
		}
		mt.AddMockResponses(first, getMore, updateSuccess)

		leaderboard := Leaderboard{
			Round:     "test_round",
			UpdatedAt: time.Now(),
			Entries: []LeaderboardEntry{
				{
					UserID:   "user1",
					Username: "TestUser1",
					Score:    15,
				},
			},
		}

		err := store.StoreLeaderboard(leaderboard)
		assert.NoError(t, err)
	})
}

func TestStoreLeaderboard_EmptyLeaderboard(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when leaderboard is empty", func(mt *mtest.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		err := store.StoreLeaderboard(Leaderboard{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "leaderboard is empty")
	})
}

func TestStoreLeaderboard_FindOneError(t *testing.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		leaderboard := Leaderboard{
			Round:     "test_round",
			UpdatedAt: time.Now(),
			Entries: []LeaderboardEntry{
				{UserID: "user1", Username: "TestUser1", Score: 10},
			},
		}

		err := store.StoreLeaderboard(leaderboard)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lookup for existing record failed")
	})
}

func TestStoreLeaderboard_InsertError(t *testing.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.leaderboard", mtest.FirstBatch))
		// Mock InsertOne error
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "insert failed",
		}))

		leaderboard := Leaderboard{
			Round:     "test_round",
			UpdatedAt: time.Now(),
			Entries: []LeaderboardEntry{
				{UserID: "user1", Username: "TestUser1", Score: 10},
			},
		}

		err := store.StoreLeaderboard(leaderboard)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "leaderboard insert failed")
	})
}

func TestStoreLeaderboard_UpdateError(t *testing.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning existing document
		existingDoc := mtest.CreateCursorResponse(1, "test.leaderboard", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
		})
		mt.AddMockResponses(existingDoc)
		// Mock UpdateOne error
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "update failed",
		}))

		leaderboard := Leaderboard{
			Round:     "test_round",
			UpdatedAt: time.Now(),
			Entries: []LeaderboardEntry{
				{UserID: "user1", Username: "TestUser1", Score: 10},
			},
		}

		err := store.StoreLeaderboard(leaderboard)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "leaderboard update failed")
	})
}

func TestStoreLeaderboard_MultipleEntries(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully stores leaderboard with multiple entries", func(mt *mtest.T) {
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
				Leaderboard: mt.Coll,
			},
		}

		// Mock FindOne returning no documents (new leaderboard)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.leaderboard", mtest.FirstBatch))
		// Mock InsertOne success
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		leaderboard := Leaderboard{
			Round:     "test_round",
			UpdatedAt: time.Now(),
			Entries: []LeaderboardEntry{
				{UserID: "user1", Username: "FirstPlace", Score: 100, ScoreResult: ScoreResult{Successes: 50, Pending: 0, Failed: 0}},
				{UserID: "user2", Username: "SecondPlace", Score: 90, ScoreResult: ScoreResult{Successes: 45, Pending: 0, Failed: 5}},
				{UserID: "user3", Username: "ThirdPlace", Score: 80, ScoreResult: ScoreResult{Successes: 40, Pending: 0, Failed: 10}},
				{UserID: "user4", Username: "FourthPlace", Score: 70, ScoreResult: ScoreResult{Successes: 35, Pending: 0, Failed: 15}},
				{UserID: "user5", Username: "FifthPlace", Score: 60, ScoreResult: ScoreResult{Successes: 30, Pending: 0, Failed: 20}},
			},
		}

		err := store.StoreLeaderboard(leaderboard)
		assert.NoError(t, err)
	})
}

// endregion
