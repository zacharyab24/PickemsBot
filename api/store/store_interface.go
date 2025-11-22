/* store_interface.go
 * Contains the Store interface for dependency injection and testing
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"pickems-bot/api/external"
)

// Interface defines the methods that Store implements.
// This allows for mocking in tests.
type Interface interface {
	EnsureScheduledMatches() error
	GetValidTeams() ([]string, string, error)
	StoreUserPrediction(userID string, prediction Prediction) error
	GetUserPrediction(userID string) (Prediction, error)
	GetMatchResults() (external.MatchResult, error)
	GetAllUserPredictions() ([]Prediction, error)
	FetchMatchSchedule() ([]external.ScheduledMatch, error)
	StoreMatchSchedule(matches []external.ScheduledMatch) error

	// Getter methods for accessing fields
	GetDatabase() interface{ Name() string }
	GetRound() string
	GetPage() string
	GetOptionalParams() string
	GetClient() interface{ Disconnect(context.Context) error }
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

// GetOptionalParams returns optional query parameters
func (s *Store) GetOptionalParams() string {
	return s.OptionalParams
}

// GetClient returns the MongoDB client
func (s *Store) GetClient() interface{ Disconnect(context.Context) error } {
	return s.Client
}
