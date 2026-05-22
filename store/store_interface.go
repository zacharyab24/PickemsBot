/* store_interface.go
 * Contains the Store interface for dependency injection and testing
 * Authors: Zachary Bower
 */

package store

import (
	"context"

	"pickems-bot/sources"
	"pickems-bot/tournament"
	"pickems-bot/models"
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

	// Getter methods for accessing fields
	GetDatabase() interface{ Name() string }
	GetRound() string
	GetPage() string
	GetFormat() string
	GetClient() interface{ Disconnect(context.Context) error }
	FetchAndUpdateMatchResults() error
	FetchAndUpdateMatchResultsFromJSON(jsonResponse string) error
	FetchMatchNodesFromDb() ([]sources.MatchNode, tournament.Kind, error)
	StoreLeaderboard(leaderboard Leaderboard) error
	FetchLeaderboardFromDB() ([]LeaderboardEntry, error)
}

// Ensure Store implements Interface
var _ Interface = (*Store)(nil)

// GetDatabase returns the database instance
func (s *Store) GetDatabase() interface{ Name() string } {
	return s.Database
}

// GetRound returns the tournament round name
func (s *Store) GetRound() string {
	return s.Round
}

// GetPage returns the Liquipedia page path
func (s *Store) GetPage() string {
	return s.Page
}

// GetFormat returns the optional format override (empty string = auto-detect)
func (s *Store) GetFormat() string {
	return s.Format
}

// GetClient returns the MongoDB client
func (s *Store) GetClient() interface{ Disconnect(context.Context) error } {
	return s.Client
}
