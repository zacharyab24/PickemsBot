/* match_results_test.go
 * Contains unit tests for match_results.go functions
 * Authors: Zachary Bower
 * AI-Generated: Additional tests for FetchMatchResultsFromDb, GetMatchResults, StoreMatchResults
 */

package store

import (
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// region DetermineTTL tests

// TestDetermineTTL_NoMatches tests with empty match list
func TestDetermineTTL_NoMatches(t *testing.T) {
	matches := []external.ScheduledMatch{}

	ttl := DetermineTTL(matches)

	// Should return normal TTL (30 minutes from now)
	now := time.Now().Unix()
	expectedTTL := time.Now().Add(30 * time.Minute).Unix()

	// Allow 2 second variance for test execution time
	assert.InDelta(t, expectedTTL, ttl, 2)
	assert.Greater(t, ttl, now)
}

// TestDetermineTTL_OngoingMatchBO1 tests with ongoing BO1 match
func TestDetermineTTL_OngoingMatchBO1(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 1800, // Started 30 minutes ago
			BestOf:    "1",        // BO1: 90 min duration
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return short TTL (3 minutes from now)
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_OngoingMatchBO3 tests with ongoing BO3 match
func TestDetermineTTL_OngoingMatchBO3(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 3600, // Started 1 hour ago
			BestOf:    "3",        // BO3: 4 hour duration
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return short TTL (3 minutes from now)
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_OngoingMatchBO5 tests with ongoing BO5 match
func TestDetermineTTL_OngoingMatchBO5(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 7200, // Started 2 hours ago
			BestOf:    "5",        // BO5: 6 hour duration
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return short TTL (3 minutes from now)
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_FutureMatch tests with match starting in the future
func TestDetermineTTL_FutureMatch(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now + 3600, // Starts in 1 hour
			BestOf:    "3",
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return normal TTL (30 minutes from now)
	expectedTTL := time.Now().Add(30 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_FinishedMatch tests with match that should be finished
func TestDetermineTTL_FinishedMatch(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 18000, // Started 5 hours ago
			BestOf:    "3",         // BO3: 4 hour duration, so finished 1 hour ago
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return normal TTL (30 minutes from now)
	expectedTTL := time.Now().Add(30 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_MultipleMatches_OneOngoing tests with multiple matches where one is ongoing
func TestDetermineTTL_MultipleMatches_OneOngoing(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 18000, // Finished
			BestOf:    "3",
			Finished:  true,
		},
		{
			Team1:     "TeamC",
			Team2:     "TeamD",
			EpochTime: now - 1800, // Ongoing (started 30 min ago)
			BestOf:    "3",
			Finished:  false,
		},
		{
			Team1:     "TeamE",
			Team2:     "TeamF",
			EpochTime: now + 3600, // Future
			BestOf:    "3",
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return short TTL (3 minutes) because one match is ongoing
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_MultipleMatches_NoneOngoing tests with multiple matches but none ongoing
func TestDetermineTTL_MultipleMatches_NoneOngoing(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 18000, // Finished
			BestOf:    "3",
			Finished:  true,
		},
		{
			Team1:     "TeamC",
			Team2:     "TeamD",
			EpochTime: now + 3600, // Future
			BestOf:    "3",
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return normal TTL (30 minutes)
	expectedTTL := time.Now().Add(30 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_InvalidBestOf tests with invalid BestOf value (defaults to 3 hours)
func TestDetermineTTL_InvalidBestOf(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 1800, // Started 30 min ago
			BestOf:    "invalid",  // Invalid value, should default to 3 hours
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return short TTL (3 minutes) because match is ongoing with default duration
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_EmptyBestOf tests with empty BestOf value (defaults to 3 hours)
func TestDetermineTTL_EmptyBestOf(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 1800, // Started 30 min ago
			BestOf:    "",         // Empty value, should default to 3 hours
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return short TTL (3 minutes) because match is ongoing
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_BO1AtEdgeOfCompletion tests BO1 match at exactly estimated finish time
func TestDetermineTTL_BO1AtEdgeOfCompletion(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - (90 * 60), // Started exactly 90 minutes ago (BO1 duration)
			BestOf:    "1",
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// At the edge, should still be considered ongoing (now <= finishedTime)
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_BO3AtEdgeOfCompletion tests BO3 match at exactly estimated finish time
func TestDetermineTTL_BO3AtEdgeOfCompletion(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - (4 * 60 * 60), // Started exactly 4 hours ago (BO3 duration)
			BestOf:    "3",
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// At the edge, should still be considered ongoing
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_BO5AtEdgeOfCompletion tests BO5 match at exactly estimated finish time
func TestDetermineTTL_BO5AtEdgeOfCompletion(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - (6 * 60 * 60), // Started exactly 6 hours ago (BO5 duration)
			BestOf:    "5",
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// At the edge, should still be considered ongoing
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_MatchJustStarted tests match that just started (within 1 minute)
func TestDetermineTTL_MatchJustStarted(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 30, // Started 30 seconds ago
			BestOf:    "3",
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return short TTL (3 minutes)
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// TestDetermineTTL_CaseInsensitiveBestOf tests case insensitivity for BestOf values
func TestDetermineTTL_CaseInsensitiveBestOf(t *testing.T) {
	now := time.Now().Unix()
	matches := []external.ScheduledMatch{
		{
			Team1:     "TeamA",
			Team2:     "TeamB",
			EpochTime: now - 1800, // Started 30 min ago
			BestOf:    "3",        // Lowercase used in switch statement
			Finished:  false,
		},
	}

	ttl := DetermineTTL(matches)

	// Should return short TTL (3 minutes)
	expectedTTL := time.Now().Add(3 * time.Minute).Unix()
	assert.InDelta(t, expectedTTL, ttl, 2)
}

// endregion

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

		swissResult, ok := result.(SwissResultRecord)
		require.True(t, ok)
		assert.Equal(t, "swiss", swissResult.GetType())
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

		elimResult, ok := result.(EliminationResultRecord)
		require.True(t, ok)
		assert.Equal(t, "single-elimination", elimResult.GetType())
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
		assert.Equal(t, "swiss", result.GetType())

		swissResult, ok := result.(external.SwissResult)
		require.True(t, ok)
		assert.Equal(t, "3-0", swissResult.Scores["Team A"])
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
		assert.Equal(t, "single-elimination", result.GetType())

		elimResult, ok := result.(external.EliminationResult)
		require.True(t, ok)
		assert.Equal(t, "semifinal", elimResult.Progression["Team X"].Round)
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

		matchResult := external.SwissResult{
			Scores: map[string]string{
				"Team A": "3-0",
				"Team B": "3-1",
			},
		}
		upcomingMatches := CreateSampleScheduledMatches()

		err := store.StoreMatchResults(matchResult, upcomingMatches)
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

		matchResult := external.EliminationResult{
			Progression: map[string]shared.TeamProgress{
				"Team X": {Round: "semifinal", Status: "advanced"},
				"Team Y": {Round: "quarterfinal", Status: "eliminated"},
			},
		}
		upcomingMatches := CreateSampleScheduledMatches()

		err := store.StoreMatchResults(matchResult, upcomingMatches)
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

		matchResult := external.SwissResult{
			Scores: map[string]string{
				"Team A": "3-0",
			},
		}

		err := store.StoreMatchResults(matchResult, []external.ScheduledMatch{})
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

		matchResult := external.SwissResult{
			Scores: map[string]string{"Team A": "3-0"},
		}

		err := store.StoreMatchResults(matchResult, []external.ScheduledMatch{})
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

		matchResult := external.SwissResult{
			Scores: map[string]string{"Team A": "3-0"},
		}

		err := store.StoreMatchResults(matchResult, []external.ScheduledMatch{})
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

		matchResult := external.SwissResult{
			Scores: map[string]string{"Team A": "3-0"},
		}

		err := store.StoreMatchResults(matchResult, []external.ScheduledMatch{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update match result")
	})
}

// endregion
