/* store_interface.go
 * Contains the Store interface for dependency injection and testing
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"pickems-bot/api/external"
)

// StoreInterface defines the methods that Store implements
// This allows for mocking in tests
type StoreInterface interface {
	EnsureScheduledMatches() error
	GetValidTeams() ([]string, string, error)
	StoreUserPrediction(userId string, prediction Prediction) error
	GetUserPrediction(userId string) (Prediction, error)
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

// Ensure Store implements StoreInterface
var _ StoreInterface = (*Store)(nil)

// Implement getter methods for Store
func (s *Store) GetDatabase() interface{ Name() string } {
	return s.Database
}

func (s *Store) GetRound() string {
	return s.Round
}

func (s *Store) GetPage() string {
	return s.Page
}

func (s *Store) GetOptionalParams() string {
	return s.OptionalParams
}

func (s *Store) GetClient() interface{ Disconnect(context.Context) error } {
	return s.Client
}
