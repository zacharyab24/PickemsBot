/* user_predictions_test.go
 * Contains unit tests for user_predictions.go
 * AI-Generated
 */

package store

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestStoreUserPrediction_InsertNew(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully inserts new prediction", func(mt *mtest.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock FindOne returning no documents (new prediction)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.predictions", mtest.FirstBatch))
		// Mock InsertOne success
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		prediction := Prediction{
			UserID:   "user123",
			Username: "testuser",
			Format:   "swiss",
			Round:    "test_round",
			Win:      []string{"Team A", "Team B"},
			Advance:  []string{"Team C", "Team D"},
			Lose:     []string{"Team E", "Team F"},
		}

		err := store.StoreUserPrediction("user123", prediction)
		assert.NoError(t, err)
	})
}

func TestStoreUserPrediction_UpdateExisting(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully updates existing prediction", func(mt *mtest.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock FindOne returning an existing document - need cursor response with getMore
		first := mtest.CreateCursorResponse(1, "test.predictions", mtest.FirstBatch, bson.D{
			{Key: "userid", Value: "user123"},
			{Key: "round", Value: "test_round"},
		})
		getMore := mtest.CreateCursorResponse(0, "test.predictions", mtest.NextBatch)
		// Mock UpdateOne success
		updateSuccess := bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
		}
		mt.AddMockResponses(first, getMore, updateSuccess)

		prediction := Prediction{
			UserID:   "user123",
			Username: "testuser",
			Format:   "swiss",
			Round:    "test_round",
			Win:      []string{"Team X", "Team Y"},
		}

		err := store.StoreUserPrediction("user123", prediction)
		assert.NoError(t, err)
	})
}

func TestStoreUserPrediction_FindOneError(t *testing.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock FindOne returning an error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		prediction := Prediction{
			UserID: "user123",
			Round:  "test_round",
		}

		err := store.StoreUserPrediction("user123", prediction)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lookup for existing prediction failed")
	})
}

func TestStoreUserPrediction_InsertError(t *testing.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.predictions", mtest.FirstBatch))
		// Mock InsertOne error
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "duplicate key error",
		}))

		prediction := Prediction{
			UserID: "user123",
			Round:  "test_round",
		}

		err := store.StoreUserPrediction("user123", prediction)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert new user prediction")
	})
}

func TestStoreUserPrediction_UpdateError(t *testing.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock FindOne returning an existing document
		existingDoc := mtest.CreateCursorResponse(1, "test.predictions", mtest.FirstBatch, bson.D{
			{Key: "userid", Value: "user123"},
			{Key: "round", Value: "test_round"},
		})
		mt.AddMockResponses(existingDoc)
		// Mock UpdateOne error
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "update failed",
		}))

		prediction := Prediction{
			UserID: "user123",
			Round:  "test_round",
		}

		err := store.StoreUserPrediction("user123", prediction)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update existing user prediction")
	})
}

func TestGetUserPrediction_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully gets user prediction", func(mt *mtest.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock FindOne returning a prediction
		predictionDoc := mtest.CreateCursorResponse(1, "test.predictions", mtest.FirstBatch, bson.D{
			{Key: "userid", Value: "user123"},
			{Key: "username", Value: "testuser"},
			{Key: "format", Value: "swiss"},
			{Key: "round", Value: "test_round"},
			{Key: "win", Value: bson.A{"Team A", "Team B"}},
			{Key: "advance", Value: bson.A{"Team C", "Team D"}},
			{Key: "lose", Value: bson.A{"Team E", "Team F"}},
		})
		mt.AddMockResponses(predictionDoc)

		prediction, err := store.GetUserPrediction("user123")
		require.NoError(t, err)
		assert.Equal(t, "user123", prediction.UserID)
		assert.Equal(t, "testuser", prediction.Username)
		assert.Equal(t, "swiss", prediction.Format)
		assert.Equal(t, []string{"Team A", "Team B"}, prediction.Win)
	})
}

func TestGetUserPrediction_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when prediction not found", func(mt *mtest.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.predictions", mtest.FirstBatch))

		prediction, err := store.GetUserPrediction("nonexistent")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Equal(t, Prediction{}, prediction)
	})
}

func TestGetUserPrediction_DatabaseError(t *testing.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock FindOne returning an error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		prediction, err := store.GetUserPrediction("user123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error fetching results from db")
		assert.Equal(t, Prediction{}, prediction)
	})
}

func TestGetAllUserPredictions_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully gets all user predictions", func(mt *mtest.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock Find returning multiple predictions
		first := mtest.CreateCursorResponse(1, "test.predictions", mtest.FirstBatch, bson.D{
			{Key: "userid", Value: "user1"},
			{Key: "username", Value: "testuser1"},
			{Key: "round", Value: "test_round"},
		})
		second := mtest.CreateCursorResponse(1, "test.predictions", mtest.NextBatch, bson.D{
			{Key: "userid", Value: "user2"},
			{Key: "username", Value: "testuser2"},
			{Key: "round", Value: "test_round"},
		})
		killCursors := mtest.CreateCursorResponse(0, "test.predictions", mtest.NextBatch)
		mt.AddMockResponses(first, second, killCursors)

		predictions, err := store.GetAllUserPredictions()
		require.NoError(t, err)
		assert.Len(t, predictions, 2)
		assert.Equal(t, "user1", predictions[0].UserID)
		assert.Equal(t, "user2", predictions[1].UserID)
	})
}

func TestGetAllUserPredictions_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns empty slice when no predictions", func(mt *mtest.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock Find returning empty cursor
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.predictions", mtest.FirstBatch))

		predictions, err := store.GetAllUserPredictions()
		require.NoError(t, err)
		assert.Empty(t, predictions)
	})
}

func TestGetAllUserPredictions_FindError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when find fails", func(mt *mtest.T) {
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
				Predictions: mt.Coll,
			},
		}

		// Mock Find returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "find failed",
		}))

		predictions, err := store.GetAllUserPredictions()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error fetching results from db")
		assert.Nil(t, predictions)
	})
}

func TestGetValidTeams_Swiss(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns valid teams for swiss format", func(mt *mtest.T) {
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
				MatchResults: mt.Coll,
			},
		}

		// Mock FetchMatchResultsFromDb returning swiss results
		swissDoc := mtest.CreateCursorResponse(1, "test.match_results", mtest.FirstBatch, bson.D{
			{Key: "type", Value: "swiss"},
			{Key: "round", Value: "test_round"},
			{Key: "teams", Value: bson.D{
				{Key: "Team A", Value: "3-0"},
				{Key: "Team B", Value: "3-1"},
				{Key: "Team C", Value: "3-2"},
			}},
		})
		mt.AddMockResponses(swissDoc)

		teams, format, err := store.GetValidTeams()
		require.NoError(t, err)
		assert.Equal(t, "swiss", format)
		assert.Len(t, teams, 3)
		assert.Contains(t, teams, "Team A")
		assert.Contains(t, teams, "Team B")
		assert.Contains(t, teams, "Team C")
	})
}

func TestGetValidTeams_Elimination(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns valid teams for elimination format", func(mt *mtest.T) {
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
				MatchResults: mt.Coll,
			},
		}

		// Mock FetchMatchResultsFromDb returning elimination results
		elimDoc := mtest.CreateCursorResponse(1, "test.match_results", mtest.FirstBatch, bson.D{
			{Key: "type", Value: "single-elimination"},
			{Key: "round", Value: "test_round"},
			{Key: "teams", Value: bson.D{
				{Key: "Team X", Value: bson.D{
					{Key: "round", Value: "semifinal"},
					{Key: "status", Value: "advanced"},
				}},
				{Key: "Team Y", Value: bson.D{
					{Key: "round", Value: "quarterfinal"},
					{Key: "status", Value: "eliminated"},
				}},
			}},
		})
		mt.AddMockResponses(elimDoc)

		teams, format, err := store.GetValidTeams()
		require.NoError(t, err)
		assert.Equal(t, "single-elimination", format)
		assert.Len(t, teams, 2)
		assert.Contains(t, teams, "Team X")
		assert.Contains(t, teams, "Team Y")
	})
}

func TestGetValidTeams_FetchError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when fetch fails", func(mt *mtest.T) {
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
				MatchResults: mt.Coll,
			},
		}

		// Mock FetchMatchResultsFromDb returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "fetch failed",
		}))

		teams, format, err := store.GetValidTeams()
		assert.Error(t, err)
		assert.Nil(t, teams)
		assert.Empty(t, format)
	})
}

// Integration test helper to verify store operations work together
func TestStoreUserPrediction_Integration(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("insert and retrieve prediction workflow", func(mt *mtest.T) {
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
				Predictions: mt.Coll,
			},
		}

		prediction := Prediction{
			UserID:   "user123",
			Username: "testuser",
			Format:   "swiss",
			Round:    "test_round",
			Win:      []string{"Team A", "Team B"},
			Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
			Lose:     []string{"Team I", "Team J"},
		}

		// Mock for StoreUserPrediction (FindOne + InsertOne)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.predictions", mtest.FirstBatch))
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := store.StoreUserPrediction("user123", prediction)
		require.NoError(t, err)

		// Mock for GetUserPrediction
		mt.AddMockResponses(mtest.CreateCursorResponse(1, "test.predictions", mtest.FirstBatch, bson.D{
			{Key: "userid", Value: "user123"},
			{Key: "username", Value: "testuser"},
			{Key: "format", Value: "swiss"},
			{Key: "round", Value: "test_round"},
			{Key: "win", Value: bson.A{"Team A", "Team B"}},
		}))

		retrieved, err := store.GetUserPrediction("user123")
		require.NoError(t, err)
		assert.Equal(t, prediction.UserID, retrieved.UserID)
		assert.Equal(t, prediction.Username, retrieved.Username)
	})
}

// Test with real MongoDB (requires test database to be running)
func TestUserPredictions_WithRealDB(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip in CI environment
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires MongoDB in CI environment")
	}

	// This test requires MONGODB_TEST_URI environment variable
	mongoURI := os.Getenv("MONGO_TEST_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	store, cleanup, err := CreateTestStore(mongoURI)
	if err != nil {
		t.Skipf("Skipping test: could not connect to MongoDB: %v", err)
	}
	defer cleanup()

	t.Run("full prediction lifecycle", func(t *testing.T) {
		prediction := CreateSamplePrediction("integration_user", "IntegrationUser", "swiss", store.Round)

		// Store prediction
		err := store.StoreUserPrediction(prediction.UserID, prediction)
		require.NoError(t, err)

		// Retrieve prediction
		retrieved, err := store.GetUserPrediction(prediction.UserID)
		require.NoError(t, err)
		assert.Equal(t, prediction.UserID, retrieved.UserID)
		assert.Equal(t, prediction.Username, retrieved.Username)

		// Update prediction
		prediction.Win = []string{"Updated Team A", "Updated Team B"}
		err = store.StoreUserPrediction(prediction.UserID, prediction)
		require.NoError(t, err)

		// Verify update
		updated, err := store.GetUserPrediction(prediction.UserID)
		require.NoError(t, err)
		assert.Equal(t, []string{"Updated Team A", "Updated Team B"}, updated.Win)

		// Clean up test document
		_, err = store.Collections.Predictions.DeleteOne(context.TODO(), bson.M{"userid": prediction.UserID})
		require.NoError(t, err)
	})
}
