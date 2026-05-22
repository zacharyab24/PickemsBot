/* models_test.go
 * Contains unit tests for models.go functions
 * Authors: Zachary Bower, Claude Opus 4.5
 */

package store

import (
	"pickems-bot/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests for SwissResult/EliminationResult interface methods (GetType,
// GetRound, GetTeamNames) live in the format package alongside the types.

// region Leaderboard and LeaderboardEntry tests

// TestLeaderboardEntry_Fields tests LeaderboardEntry struct fields
func TestLeaderboardEntry_Fields(t *testing.T) {
	entry := LeaderboardEntry{
		UserID:   "user123",
		Username: "testuser",
		Score:    10,
		ScoreResult: models.ScoreResult{
			Successes: 8,
			Pending:   2,
			Failed:    0,
		},
	}

	assert.Equal(t, "user123", entry.UserID)
	assert.Equal(t, "testuser", entry.Username)
	assert.Equal(t, 10, entry.Score)
	assert.Equal(t, 8, entry.ScoreResult.Successes)
	assert.Equal(t, 2, entry.ScoreResult.Pending)
	assert.Equal(t, 0, entry.ScoreResult.Failed)
}

// TestLeaderboard_Fields tests Leaderboard struct fields
func TestLeaderboard_Fields(t *testing.T) {
	entries := []LeaderboardEntry{
		{UserID: "user1", Username: "player1", Score: 10},
		{UserID: "user2", Username: "player2", Score: 8},
	}

	leaderboard := Leaderboard{
		Round:   "test-round",
		Entries: entries,
	}

	assert.Equal(t, "test-round", leaderboard.Round)
	assert.Len(t, leaderboard.Entries, 2)
	assert.Equal(t, "user1", leaderboard.Entries[0].UserID)
	assert.Equal(t, "user2", leaderboard.Entries[1].UserID)
}

// TestLeaderboard_EmptyEntries tests Leaderboard with no entries
func TestLeaderboard_EmptyEntries(t *testing.T) {
	leaderboard := Leaderboard{
		Round:   "test-round",
		Entries: []LeaderboardEntry{},
	}

	assert.Equal(t, "test-round", leaderboard.Round)
	assert.Empty(t, leaderboard.Entries)
}

// TestScoreResult_Fields tests models.ScoreResult struct fields
func TestScoreResult_Fields(t *testing.T) {
	scoreResult := models.ScoreResult{
		Successes: 5,
		Pending:   3,
		Failed:    2,
	}

	assert.Equal(t, 5, scoreResult.Successes)
	assert.Equal(t, 3, scoreResult.Pending)
	assert.Equal(t, 2, scoreResult.Failed)
}

// TestLeaderboardEntry_InlineScoreResult tests that models.ScoreResult is properly inlined
func TestLeaderboardEntry_InlineScoreResult(t *testing.T) {
	entry := LeaderboardEntry{
		UserID:   "user123",
		Username: "testuser",
		Score:    5,
		ScoreResult: models.ScoreResult{
			Successes: 7,
			Pending:   1,
			Failed:    3,
		},
	}

	// Direct access to inline fields
	assert.Equal(t, 7, entry.Successes)
	assert.Equal(t, 1, entry.Pending)
	assert.Equal(t, 3, entry.Failed)
}

// endregion
