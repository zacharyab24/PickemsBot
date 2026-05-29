/* models.go
 * This file contain the interfaces, structs and helper functions that are used by api consumers
 * Authors: Zachary Bower
 */

package app

// ScoreResult represents the outcome of score calculation for a user's predictions
type ScoreResult struct {
	Successes int
	Pending   int
	Failed    int
}

// TournamentInfo provides metadata for the configured tournament
type TournamentInfo struct {
	TournamentName string
	Round          string
	Format         string
	NumTeams       int
}

// LeaderboardUser represents a single user on the leaderboard
type LeaderboardUser struct {
	Username  string
	Rank      int
	Successes int
	Failures  int
}

// Team represents a tournament team with its associated VRS world ranking.
type Team struct {
	Name       string
	VRSRanking int
}
