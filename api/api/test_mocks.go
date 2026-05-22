/* test_mocks.go
 * Contains mock structures and interfaces for testing the API package
 * Authors: Zachary Bower
 */

package api

import (
	"context"
	"fmt"

	"pickems-bot/api/external"
	"pickems-bot/api/format"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"

	"go.mongodb.org/mongo-driver/mongo"
)

// MockStore implements the Store interface for testing
type MockStore struct {
	// Storage for mock data
	Predictions      map[string]shared.Prediction
	MatchResults     format.MatchResult
	ScheduledMatches []external.ScheduledMatch
	ValidTeams       []string
	Format           format.Kind

	// Error injection for testing error paths
	EnsureScheduledMatchesError             error
	GetValidTeamsError                      error
	StoreUserPredictionError                error
	GetUserPredictionError                  error
	GetMatchResultsError                    error
	GetAllUserPredictionsError              error
	FetchMatchScheduleError                 error
	StoreMatchScheduleError                 error
	FetchAndUpdateMatchResultsError         error
	FetchAndUpdateMatchResultsFromJSONError error
	StoreLeaderboardError                   error
	FetchLeaderboardFromDBError             error

	// Leaderboard storage
	Leaderboard []store.LeaderboardEntry

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
func NewMockStore(kind format.Kind, round string) *MockStore {
	return &MockStore{
		Predictions:      make(map[string]shared.Prediction),
		ScheduledMatches: []external.ScheduledMatch{},
		ValidTeams:       []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J", "Team K", "Team L", "Team M", "Team N", "Team O", "Team P"},
		Format:           kind,
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
func (m *MockStore) GetValidTeams() ([]string, format.Kind, error) {
	if m.GetValidTeamsError != nil {
		return nil, "", m.GetValidTeamsError
	}
	return m.ValidTeams, m.Format, nil
}

// StoreUserPrediction mock implementation
func (m *MockStore) StoreUserPrediction(userID string, prediction shared.Prediction) error {
	if m.StoreUserPredictionError != nil {
		return m.StoreUserPredictionError
	}
	m.Predictions[userID] = prediction
	return nil
}

// GetUserPrediction mock implementation
func (m *MockStore) GetUserPrediction(userID string) (shared.Prediction, error) {
	if m.GetUserPredictionError != nil {
		return shared.Prediction{}, m.GetUserPredictionError
	}
	pred, ok := m.Predictions[userID]
	if !ok {
		return shared.Prediction{}, mongo.ErrNoDocuments
	}
	return pred, nil
}

// GetMatchResults mock implementation
func (m *MockStore) GetMatchResults() (format.MatchResult, error) {
	if m.GetMatchResultsError != nil {
		return nil, m.GetMatchResultsError
	}
	if m.MatchResults == nil {
		return format.SwissResult{Teams: make(map[string]string)}, nil
	}
	return m.MatchResults, nil
}

// GetAllUserPredictions mock implementation
func (m *MockStore) GetAllUserPredictions() ([]shared.Prediction, error) {
	if m.GetAllUserPredictionsError != nil {
		return nil, m.GetAllUserPredictionsError
	}

	var predictions []shared.Prediction
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
	m.MatchResults = format.SwissResult{
		Round: m.RoundName,
		Teams: scores,
	}
}

// SetEliminationResults sets up mock single-elimination tournament results
func (m *MockStore) SetEliminationResults(progression map[string]shared.TeamProgress) {
	m.MatchResults = format.EliminationResult{
		Round: m.RoundName,
		Teams: progression,
	}
	m.Format = format.SingleElim
	// Update valid teams from progression
	m.ValidTeams = make([]string, 0, len(progression))
	for team := range progression {
		m.ValidTeams = append(m.ValidTeams, team)
	}
}

// SetScheduleError sets an error for FetchMatchSchedule (convenience method)
func (m *MockStore) SetScheduleError(err error) {
	m.FetchMatchScheduleError = err
}

// SetScheduledMatches sets up mock scheduled matches
func (m *MockStore) SetScheduledMatches(matches []external.ScheduledMatch) {
	m.ScheduledMatches = matches
}

// GetDatabase returns the mock database interface
func (m *MockStore) GetDatabase() interface{ Name() string } {
	return m.Database
}

// GetRound returns the round name
func (m *MockStore) GetRound() string {
	return m.Round
}

// GetPage returns the page path
func (m *MockStore) GetPage() string {
	return "Test/Tournament/2025"
}

// GetFormat returns the format override (empty = auto-detect)
func (m *MockStore) GetFormat() string {
	return ""
}

// mockClient implements minimal client interface
type mockClient struct{}

func (mc *mockClient) Disconnect(ctx context.Context) error {
	return nil
}

// GetClient returns the mock MongoDB client
func (m *MockStore) GetClient() interface{ Disconnect(context.Context) error } {
	return &mockClient{}
}

// FetchAndUpdateMatchResults mock implementation
func (m *MockStore) FetchAndUpdateMatchResults() error {
	if m.FetchAndUpdateMatchResultsError != nil {
		return m.FetchAndUpdateMatchResultsError
	}
	return nil
}

// FetchAndUpdateMatchResultsFromJSON mock implementation
func (m *MockStore) FetchAndUpdateMatchResultsFromJSON(_ string) error {
	if m.FetchAndUpdateMatchResultsFromJSONError != nil {
		return m.FetchAndUpdateMatchResultsFromJSONError
	}
	return nil
}

// FetchMatchNodesFromDb mock implementation
func (m *MockStore) FetchMatchNodesFromDb() ([]external.MatchNode, format.Kind, error) {
	return nil, "", nil
}

// StoreLeaderboard mock implementation
func (m *MockStore) StoreLeaderboard(leaderboard store.Leaderboard) error {
	if m.StoreLeaderboardError != nil {
		return m.StoreLeaderboardError
	}
	m.Leaderboard = leaderboard.Entries
	return nil
}

// FetchLeaderboardFromDB mock implementation
func (m *MockStore) FetchLeaderboardFromDB() ([]store.LeaderboardEntry, error) {
	if m.FetchLeaderboardFromDBError != nil {
		return nil, m.FetchLeaderboardFromDBError
	}
	return m.Leaderboard, nil
}
