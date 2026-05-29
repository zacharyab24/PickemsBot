/* store_interface.go
 * Contains the Store interface for dependency injection and testing
 * Authors: Zachary Bower
 */

package store

import (
	"context"

	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/tournament"
)

// Interface defines the methods that Store implements.
// This allows for mocking in tests.
type Interface interface {
	EnsureScheduledMatches() error
	GetValidTeams() ([]string, tournament.Kind, error)
	StoreUserPrediction(userID string, prediction models.Prediction) error
	GetUserPrediction(userID string) (models.Prediction, error)
	GetMatchResults() (tournament.MatchResult, error)
	GetAllUserPredictions() ([]models.Prediction, error)
	FetchMatchSchedule() ([]sources.ScheduledMatch, error)
	StoreMatchSchedule(matches []sources.ScheduledMatch) error
	FetchAndStoreSchedule() error
	Ping(ctx context.Context) error

	// Getter methods for accessing fields
	GetDatabase() interface{ Name() string }
	GetRound() string
	GetClient() interface{ Disconnect(context.Context) error }
	FetchAndUpdateMatchResults() error
	FetchMatchNodesFromDb() ([]sources.MatchNode, tournament.Kind, error)
	StoreLeaderboard(leaderboard Leaderboard) error
	FetchLeaderboardFromDB() ([]LeaderboardEntry, error)
}

// Ping pings the database client to ensure its online
func (s *Store) Ping(ctx context.Context) error {
	return s.Client.Ping(ctx, nil)
}

// Ensure Store implements Interface
var _ Interface = (*Store)(nil)

// GetDatabase returns the database instance
func (s *Store) GetDatabase() interface{ Name() string } {
	return s.TournamentDatabase
}

// GetRound returns the tournament round name
func (s *Store) GetRound() string {
	return s.Round
}

// GetClient returns the MongoDB client
func (s *Store) GetClient() interface{ Disconnect(context.Context) error } {
	return s.Client
}
