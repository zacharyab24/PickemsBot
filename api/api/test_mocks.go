/* test_mocks.go
 * Contains mock structures and interfaces for testing the API package
 * Authors: Zachary Bower
 */

package api

import (
	"context"
	"fmt"
	"pickems-bot/api/external"
	"pickems-bot/api/store"

	"go.mongodb.org/mongo-driver/mongo"
)

// MockStore implements the Store interface for testing
type MockStore struct {
	// Storage for mock data
	Predictions      map[string]store.Prediction
	MatchResults     store.ResultRecord
	ScheduledMatches []external.ScheduledMatch
	ValidTeams       []string
	Format           string

	// Error injection for testing error paths
	EnsureScheduledMatchesError error
	GetValidTeamsError          error
	StoreUserPredictionError    error
	GetUserPredictionError      error
	GetMatchResultsError        error
	GetAllUserPredictionsError  error
	FetchMatchScheduleError     error
	StoreMatchScheduleError     error

	// Database and Round info
	DatabaseName string
	RoundName    string

	// Store fields needed for compatibility
	Round    string
	Database interface{ Name() string }
}

// mockDatabase implements the minimal Database interface needed for tests
type mockDatabase struct {
	name string
}

func (m *mockDatabase) Name() string {
	return m.name
}

// NewMockStore creates a new MockStore with default values
func NewMockStore(format string, round string) *MockStore {
	return &MockStore{
		Predictions:      make(map[string]store.Prediction),
		ScheduledMatches: []external.ScheduledMatch{},
		ValidTeams:       []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"},
		Format:           format,
		DatabaseName:     "test_db",
		RoundName:        round,
		Round:            round,
		Database:         &mockDatabase{name: "test_db"},
	}
}

// EnsureScheduledMatches mock implementation
func (m *MockStore) EnsureScheduledMatches() error {
	if m.EnsureScheduledMatchesError != nil {
		return m.EnsureScheduledMatchesError
	}
	if len(m.ScheduledMatches) == 0 {
		return fmt.Errorf("no scheduled matches found")
	}
	return nil
}

// GetValidTeams mock implementation
func (m *MockStore) GetValidTeams() ([]string, string, error) {
	if m.GetValidTeamsError != nil {
		return nil, "", m.GetValidTeamsError
	}
	return m.ValidTeams, m.Format, nil
}

// StoreUserPrediction mock implementation
func (m *MockStore) StoreUserPrediction(userID string, prediction store.Prediction) error {
	if m.StoreUserPredictionError != nil {
		return m.StoreUserPredictionError
	}
	m.Predictions[userID] = prediction
	return nil
}

// GetUserPrediction mock implementation
func (m *MockStore) GetUserPrediction(userID string) (store.Prediction, error) {
	if m.GetUserPredictionError != nil {
		return store.Prediction{}, m.GetUserPredictionError
	}
	pred, ok := m.Predictions[userID]
	if !ok {
		return store.Prediction{}, mongo.ErrNoDocuments
	}
	return pred, nil
}

// GetMatchResults mock implementation
func (m *MockStore) GetMatchResults() (external.MatchResult, error) {
	if m.GetMatchResultsError != nil {
		return nil, m.GetMatchResultsError
	}
	if m.MatchResults == nil {
		return external.SwissResult{Scores: make(map[string]string)}, nil
	}

	// Convert ResultRecord to MatchResult
	result, err := store.ToMatchResult(m.MatchResults)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetAllUserPredictions mock implementation
func (m *MockStore) GetAllUserPredictions() ([]store.Prediction, error) {
	if m.GetAllUserPredictionsError != nil {
		return nil, m.GetAllUserPredictionsError
	}

	var predictions []store.Prediction
	for _, pred := range m.Predictions {
		predictions = append(predictions, pred)
	}

	if len(predictions) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return predictions, nil
}

// FetchMatchSchedule mock implementation
func (m *MockStore) FetchMatchSchedule() ([]external.ScheduledMatch, error) {
	if m.FetchMatchScheduleError != nil {
		return nil, m.FetchMatchScheduleError
	}
	return m.ScheduledMatches, nil
}

// StoreMatchSchedule mock implementation
func (m *MockStore) StoreMatchSchedule(matches []external.ScheduledMatch) error {
	if m.StoreMatchScheduleError != nil {
		return m.StoreMatchScheduleError
	}
	m.ScheduledMatches = matches
	return nil
}

// Helper methods for setting up test scenarios

// SetSwissResults sets up mock Swiss tournament results
func (m *MockStore) SetSwissResults(scores map[string]string) {
	m.MatchResults = store.SwissResultRecord{
		Round: m.RoundName,
		TTL:   9999999999,
		Teams: scores,
	}
}

// SetEliminationResults sets up mock single-elimination tournament results
func (m *MockStore) SetEliminationResults(progression map[string]interface{}) {
	// Implementation would convert progression to proper format
}

// SetScheduledMatches sets up mock scheduled matches
func (m *MockStore) SetScheduledMatches(matches []external.ScheduledMatch) {
	m.ScheduledMatches = matches
}

// Implement getter methods for StoreInterface
func (m *MockStore) GetDatabase() interface{ Name() string } {
	return m.Database
}

func (m *MockStore) GetRound() string {
	return m.Round
}

func (m *MockStore) GetPage() string {
	return "Test/Tournament/2025"
}

func (m *MockStore) GetOptionalParams() string {
	return ""
}

// mockClient implements minimal client interface
type mockClient struct{}

func (mc *mockClient) Disconnect(ctx context.Context) error {
	return nil
}

func (m *MockStore) GetClient() interface{ Disconnect(context.Context) error } {
	return &mockClient{}
}
