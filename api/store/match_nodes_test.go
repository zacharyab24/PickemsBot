/* match_nodes_test.go
 * Contains unit and integration tests for match_nodes.go functions
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"os"
	"testing"

	"pickems-bot/api/external"
	"pickems-bot/api/format"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// region StoreMatchNodes tests

func TestStoreMatchNodes_Insert(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully inserts new match nodes", func(mt *mtest.T) {
		store := &Store{
			Round: "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchNodes: mt.Coll,
			},
		}

		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.match_nodes", mtest.FirstBatch))
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		nodes := []external.MatchNode{
			{ID: "abc_0001", Team1: "Team A", Team2: "Team B", Winner: "Team A", Score: "2-0"},
		}
		err := store.StoreMatchNodes(nodes, format.Swiss)
		assert.NoError(t, err)
	})
}

func TestStoreMatchNodes_Update(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully updates existing match nodes", func(mt *mtest.T) {
		store := &Store{
			Round: "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchNodes: mt.Coll,
			},
		}

		first := mtest.CreateCursorResponse(1, "test.match_nodes", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
		})
		getMore := mtest.CreateCursorResponse(0, "test.match_nodes", mtest.NextBatch)
		updateSuccess := bson.D{{Key: "ok", Value: 1}, {Key: "nModified", Value: 1}}
		mt.AddMockResponses(first, getMore, updateSuccess)

		nodes := []external.MatchNode{
			{ID: "abc_0001", Team1: "Team A", Team2: "Team B", Winner: "Team B", Score: "1-2"},
		}
		err := store.StoreMatchNodes(nodes, format.Swiss)
		assert.NoError(t, err)
	})
}

func TestStoreMatchNodes_FindOneError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when lookup fails", func(mt *mtest.T) {
		store := &Store{
			Round: "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchNodes: mt.Coll,
			},
		}

		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		err := store.StoreMatchNodes([]external.MatchNode{}, format.Swiss)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lookup for existing match nodes record failed")
	})
}

// endregion

// region FetchMatchNodesFromDb tests

func TestFetchMatchNodesFromDb_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully fetches match nodes", func(mt *mtest.T) {
		store := &Store{
			Round: "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchNodes: mt.Coll,
			},
		}

		doc := mtest.CreateCursorResponse(1, "test.match_nodes", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
			{Key: "format", Value: "swiss"},
			{Key: "nodes", Value: bson.A{
				bson.D{
					{Key: "id", Value: "abc_0001"},
					{Key: "team1", Value: "Team A"},
					{Key: "team2", Value: "Team B"},
					{Key: "winner", Value: "Team A"},
					{Key: "score", Value: "2-0"},
					{Key: "section", Value: "Round 1"},
				},
			}},
		})
		mt.AddMockResponses(doc)

		nodes, err := store.FetchMatchNodesFromDb()
		require.NoError(t, err)
		require.Len(t, nodes, 1)
		assert.Equal(t, "abc_0001", nodes[0].ID)
		assert.Equal(t, "Team A", nodes[0].Team1)
		assert.Equal(t, "Team B", nodes[0].Team2)
		assert.Equal(t, "Team A", nodes[0].Winner)
		assert.Equal(t, "2-0", nodes[0].Score)
		assert.Equal(t, "Round 1", nodes[0].Section)
	})
}

func TestFetchMatchNodesFromDb_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns ErrNoDocuments when round not found", func(mt *mtest.T) {
		store := &Store{
			Round: "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchNodes: mt.Coll,
			},
		}

		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.match_nodes", mtest.FirstBatch))

		nodes, err := store.FetchMatchNodesFromDb()
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Nil(t, nodes)
	})
}

// endregion

// region integration test

func TestMatchNodes_WithRealDB(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires MongoDB in CI environment")
	}

	mongoURI := os.Getenv("MONGO_TEST_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	store, cleanup, err := CreateTestStore(mongoURI)
	if err != nil {
		t.Skipf("Skipping test: could not connect to MongoDB: %v", err)
	}
	defer cleanup()

	nodes := []external.MatchNode{
		{ID: "test_0001", Team1: "Team A", Team2: "Team B", Winner: "Team A", Score: "2-1", Section: "Round 1"},
		{ID: "test_0002", Team1: "Team C", Team2: "Team D", Winner: "TBD", Score: "", Section: "Round 1"},
	}

	t.Run("store and fetch match nodes", func(t *testing.T) {
		err := store.StoreMatchNodes(nodes, format.Swiss)
		require.NoError(t, err)

		fetched, err := store.FetchMatchNodesFromDb()
		require.NoError(t, err)
		require.Len(t, fetched, 2)

		assert.Equal(t, "test_0001", fetched[0].ID)
		assert.Equal(t, "Team A", fetched[0].Team1)
		assert.Equal(t, "Team B", fetched[0].Team2)
		assert.Equal(t, "Team A", fetched[0].Winner)
		assert.Equal(t, "2-1", fetched[0].Score)

		assert.Equal(t, "test_0002", fetched[1].ID)
		assert.Equal(t, "TBD", fetched[1].Winner)
		assert.Equal(t, "", fetched[1].Score)
	})

	t.Run("update existing match nodes", func(t *testing.T) {
		updated := []external.MatchNode{
			{ID: "test_0001", Team1: "Team A", Team2: "Team B", Winner: "Team B", Score: "1-2"},
		}

		err := store.StoreMatchNodes(updated, format.Swiss)
		require.NoError(t, err)

		fetched, err := store.FetchMatchNodesFromDb()
		require.NoError(t, err)
		require.Len(t, fetched, 1)
		assert.Equal(t, "Team B", fetched[0].Winner)
		assert.Equal(t, "1-2", fetched[0].Score)
	})

	// Clean up
	_, err = store.Collections.MatchNodes.DeleteMany(context.TODO(), bson.M{"round": store.Round})
	require.NoError(t, err)
}

// endregion
