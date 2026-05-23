/* test_mocks.go
 * Contains mock structures and interfaces for testing the App package
 * Authors: Zachary Bower
 */

package app

import (
	"context"
	"fmt"

	"pickems-bot/sources"
	"pickems-bot/tournament"
	"pickems-bot/models"
	"pickems-bot/store"

	"go.mongodb.org/mongo-driver/mongo"
)

// MockStore implements the Store interface for testing
type MockStore struct {
	// Storage for mock data
	Predictions      map[string]models.Prediction
	MatchResults     tournament.MatchResult
	ScheduledMatches []sources.ScheduledMatch
	ValidTeams       []string
	Format           tournament.Kind

	// Error injection for testing error paths
	EnsureScheduledMatchesError             error
	GetValidTeamsError                      error
	StoreUserPredictionError                error
	GetUserPredictionError                  error
	GetMatchResultsError                    error
	GetAllUserPredictionsError              error
	FetchMatchScheduleError                 error
	StoreMatchScheduleError                 error
	FetchAndUpdateMatchResultsError error
	StoreLeaderboardError           error
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
func NewMockStore(kind tournament.Kind, round string) *MockStore {
	return &MockStore{
		Predictions:      make(map[string]models.Prediction),
		ScheduledMatches: []sources.ScheduledMatch{},
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
func (m *MockStore) GetValidTeams() ([]string, tournament.Kind, error) {
	if m.GetValidTeamsError != nil {
		return nil, "", m.GetValidTeamsError
	}
	return m.ValidTeams, m.Format, nil
}

// StoreUserPrediction mock implementation
func (m *MockStore) StoreUserPrediction(userID string, prediction models.Prediction) error {
	if m.StoreUserPredictionError != nil {
		return m.StoreUserPredictionError
	}
	m.Predictions[userID] = prediction
	return nil
}

// GetUserPrediction mock implementation
func (m *MockStore) GetUserPrediction(userID string) (models.Prediction, error) {
	if m.GetUserPredictionError != nil {
		return models.Prediction{}, m.GetUserPredictionError
	}
	pred, ok := m.Predictions[userID]
	if !ok {
		return models.Prediction{}, mongo.ErrNoDocuments
	}
	return pred, nil
}

// GetMatchResults mock implementation
func (m *MockStore) GetMatchResults() (tournament.MatchResult, error) {
	if m.GetMatchResultsError != nil {
		return nil, m.GetMatchResultsError
	}
	if m.MatchResults == nil {
		return tournament.SwissResult{Teams: make(map[string]string)}, nil
	}
	return m.MatchResults, nil
}

// GetAllUserPredictions mock implementation
func (m *MockStore) GetAllUserPredictions() ([]models.Prediction, error) {
	if m.GetAllUserPredictionsError != nil {
		return nil, m.GetAllUserPredictionsError
	}

	var predictions []models.Prediction
	for _, pred := range m.Predictions {
		predictions = append(predictions, pred)
	}

	if len(predictions) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return predictions, nil
}

// FetchMatchSchedule mock implementation
func (m *MockStore) FetchMatchSchedule() ([]sources.ScheduledMatch, error) {
	if m.FetchMatchScheduleError != nil {
		return nil, m.FetchMatchScheduleError
	}
	return m.ScheduledMatches, nil
}

// StoreMatchSchedule mock implementation
func (m *MockStore) StoreMatchSchedule(matches []sources.ScheduledMatch) error {
	if m.StoreMatchScheduleError != nil {
		return m.StoreMatchScheduleError
	}
	m.ScheduledMatches = matches
	return nil
}

// Helper methods for setting up test scenarios

// SetSwissResults sets up mock Swiss tournament results
func (m *MockStore) SetSwissResults(scores map[string]string) {
	m.MatchResults = tournament.SwissResult{
		Round: m.RoundName,
		Teams: scores,
	}
}

// SetEliminationResults sets up mock single-elimination tournament results
func (m *MockStore) SetEliminationResults(progression map[string]models.TeamProgress) {
	m.MatchResults = tournament.EliminationResult{
		Round: m.RoundName,
		Teams: progression,
	}
	m.Format = tournament.SingleElim
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
func (m *MockStore) SetScheduledMatches(matches []sources.ScheduledMatch) {
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

// FetchAndStoreSchedule mock implementation
func (m *MockStore) FetchAndStoreSchedule() error {
	return nil
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

// FetchMatchNodesFromDb mock implementation
func (m *MockStore) FetchMatchNodesFromDb() ([]sources.MatchNode, tournament.Kind, error) {
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
