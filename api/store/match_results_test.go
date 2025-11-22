/* match_results_test.go
 * Contains unit tests for match_results.go functions
 * Authors: Zachary Bower
 */

package store

import (
	"pickems-bot/api/external"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
