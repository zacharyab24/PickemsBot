/* match_results_test.go
 * Contains unit tests for match_results.go functions
 * Authors: Zachary Bower
 * AI-Generated: Additional tests for FetchMatchResultsFromDb, GetMatchResults, StoreMatchResults
 */

package store

import (
	"testing"

	"pickems-bot/tournament"
	"pickems-bot/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// region FetchMatchResultsFromDb tests

func TestFetchMatchResultsFromDb_Swiss(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully fetches swiss results", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning swiss results
		swissDoc := mtest.CreateCursorResponse(1, "test.match_results", mtest.FirstBatch, bson.D{
			{Key: "type", Value: "swiss"},
			{Key: "round", Value: "test_round"},
			{Key: "ttl", Value: int64(1700000000)},
			{Key: "teams", Value: bson.D{
				{Key: "Team A", Value: "3-0"},
				{Key: "Team B", Value: "3-1"},
				{Key: "Team C", Value: "2-3"},
			}},
		})
		mt.AddMockResponses(swissDoc)

		result, err := store.FetchMatchResultsFromDb()
		require.NoError(t, err)
		require.NotNil(t, result)

		swissResult, ok := result.(tournament.SwissResult)
		require.True(t, ok)
		assert.Equal(t, tournament.Swiss, swissResult.GetType())
		assert.Equal(t, "test_round", swissResult.GetRound())
		assert.Equal(t, "3-0", swissResult.Teams["Team A"])
		assert.Equal(t, "3-1", swissResult.Teams["Team B"])
	})
}

func TestFetchMatchResultsFromDb_Elimination(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully fetches elimination results", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning elimination results
		elimDoc := mtest.CreateCursorResponse(1, "test.match_results", mtest.FirstBatch, bson.D{
			{Key: "type", Value: "single-elimination"},
			{Key: "round", Value: "test_round"},
			{Key: "ttl", Value: int64(1700000000)},
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

		result, err := store.FetchMatchResultsFromDb()
		require.NoError(t, err)
		require.NotNil(t, result)

		elimResult, ok := result.(tournament.EliminationResult)
		require.True(t, ok)
		assert.Equal(t, tournament.SingleElim, elimResult.GetType())
		assert.Equal(t, "test_round", elimResult.GetRound())
		assert.Equal(t, "semifinal", elimResult.Teams["Team X"].Round)
		assert.Equal(t, "advanced", elimResult.Teams["Team X"].Status)
	})
}

func TestFetchMatchResultsFromDb_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when no documents found", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.match_results", mtest.FirstBatch))

		result, err := store.FetchMatchResultsFromDb()
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Nil(t, result)
	})
}

func TestFetchMatchResultsFromDb_DatabaseError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error on database failure", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		result, err := store.FetchMatchResultsFromDb()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error fetching results from db")
		assert.Nil(t, result)
	})
}

func TestFetchMatchResultsFromDb_MissingType(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when type field is missing", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning document without type field
		docWithoutType := mtest.CreateCursorResponse(1, "test.match_results", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
			{Key: "teams", Value: bson.D{}},
		})
		mt.AddMockResponses(docWithoutType)

		result, err := store.FetchMatchResultsFromDb()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid `type` field")
		assert.Nil(t, result)
	})
}

func TestFetchMatchResultsFromDb_UnknownType(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error for unknown result type", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning document with unknown type
		unknownTypeDoc := mtest.CreateCursorResponse(1, "test.match_results", mtest.FirstBatch, bson.D{
			{Key: "type", Value: "unknown-format"},
			{Key: "round", Value: "test_round"},
		})
		mt.AddMockResponses(unknownTypeDoc)

		result, err := store.FetchMatchResultsFromDb()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown match result type: unknown-format")
		assert.Nil(t, result)
	})
}

// endregion

// region GetMatchResults tests

func TestGetMatchResults_SwissSuccess(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully gets swiss match results and converts to MatchResult", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
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
			}},
		})
		mt.AddMockResponses(swissDoc)

		result, err := store.GetMatchResults()
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, tournament.Swiss, result.GetType())

		swissRecord, ok := result.(tournament.SwissResult)
		require.True(t, ok)
		assert.Equal(t, "3-0", swissRecord.Teams["Team A"])
	})
}

func TestGetMatchResults_EliminationSuccess(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully gets elimination match results and converts to MatchResult", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
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
			}},
		})
		mt.AddMockResponses(elimDoc)

		result, err := store.GetMatchResults()
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, tournament.SingleElim, result.GetType())

		elimRecord, ok := result.(tournament.EliminationResult)
		require.True(t, ok)
		assert.Equal(t, "semifinal", elimRecord.Teams["Team X"].Round)
	})
}

func TestGetMatchResults_FetchError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when fetch fails", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock fetch error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "fetch failed",
		}))

		result, err := store.GetMatchResults()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error occured getting match results from db")
		assert.Nil(t, result)
	})
}

// endregion

// region StoreMatchResults tests

func TestStoreMatchResults_InsertSwiss(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully inserts new swiss match results", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning no documents (new record)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.match_results", mtest.FirstBatch))
		// Mock InsertOne success
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		matchResult := tournament.SwissResult{
			Teams: map[string]string{
				"Team A": "3-0",
				"Team B": "3-1",
			},
		}

		err := store.StoreMatchResults(matchResult)
		assert.NoError(t, err)
	})
}

func TestStoreMatchResults_InsertElimination(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully inserts new elimination match results", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning no documents (new record)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.match_results", mtest.FirstBatch))
		// Mock InsertOne success
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		matchResult := tournament.EliminationResult{
			Teams: map[string]models.TeamProgress{
				"Team X": {Round: "semifinal", Status: "advanced"},
				"Team Y": {Round: "quarterfinal", Status: "eliminated"},
			},
		}

		err := store.StoreMatchResults(matchResult)
		assert.NoError(t, err)
	})
}

func TestStoreMatchResults_UpdateExisting(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("successfully updates existing match results", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning existing document - need cursor response with getMore
		first := mtest.CreateCursorResponse(1, "test.match_results", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
		})
		getMore := mtest.CreateCursorResponse(0, "test.match_results", mtest.NextBatch)
		// Mock UpdateOne success
		updateSuccess := bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
		}
		mt.AddMockResponses(first, getMore, updateSuccess)

		matchResult := tournament.SwissResult{
			Teams: map[string]string{
				"Team A": "3-0",
			},
		}

		err := store.StoreMatchResults(matchResult)
		assert.NoError(t, err)
	})
}

func TestStoreMatchResults_FindOneError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when FindOne fails", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning error
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		matchResult := tournament.SwissResult{
			Teams: map[string]string{"Team A": "3-0"},
		}

		err := store.StoreMatchResults(matchResult)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lookup for existing record failed")
	})
}

func TestStoreMatchResults_InsertError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when insert fails", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning no documents
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.match_results", mtest.FirstBatch))
		// Mock InsertOne error
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "insert failed",
		}))

		matchResult := tournament.SwissResult{
			Teams: map[string]string{"Team A": "3-0"},
		}

		err := store.StoreMatchResults(matchResult)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert new match result")
	})
}

func TestStoreMatchResults_UpdateError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error when update fails", func(mt *mtest.T) {
		store := &Store{
			Client:   mt.Client,
			Database: mt.DB,
			Round:    "test_round",
			Collections: struct {
				Predictions   *mongo.Collection
				MatchResults  *mongo.Collection
				MatchNodes    *mongo.Collection
				MatchSchedule *mongo.Collection
				Leaderboard   *mongo.Collection
			}{
				MatchResults: mt.Coll,
			},
		}

		// Mock FindOne returning existing document
		existingDoc := mtest.CreateCursorResponse(1, "test.match_results", mtest.FirstBatch, bson.D{
			{Key: "round", Value: "test_round"},
		})
		mt.AddMockResponses(existingDoc)
		// Mock UpdateOne error
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "update failed",
		}))

		matchResult := tournament.SwissResult{
			Teams: map[string]string{"Team A": "3-0"},
		}

		err := store.StoreMatchResults(matchResult)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update match result")
	})
}

// endregion
