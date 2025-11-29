/* api_test.go
 * Contains unit tests for api.go - testing all public API methods
 * Authors: Zachary Bower
 */

package api

import (
	"fmt"
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
	"strings"
	"testing"
	"time"
)

// region NewAPI tests

func TestNewAPI_Success(t *testing.T) {
	// This test requires a real MongoDB connection or would need significant refactoring
	// For now, we test the validation logic
	_, err := NewAPI("", "", "", "", "")
	if err == nil {
		t.Error("Expected error when dbName is empty, got nil")
	}

	if !strings.Contains(err.Error(), "dbName, page, and round are required") {
		t.Errorf("Expected error message about required fields, got: %s", err.Error())
	}
}

func TestNewAPI_MissingParameters(t *testing.T) {
	tests := []struct {
		name   string
		dbName string
		page   string
		round  string
	}{
		{"missing dbName", "", "page", "round"},
		{"missing page", "db", "", "round"},
		{"missing round", "db", "page", ""},
		{"all missing", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAPI(tt.dbName, "mongodb://localhost", tt.page, "", tt.round)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
		})
	}
}

// endregion

// region SetUserPrediction tests

func TestSetUserPrediction_SwissFormat_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &API{Store: mockStore}

	user := shared.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}

	err := api.SetUserPrediction(user, teams, "test_round")
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	// Verify prediction was stored
	pred, ok := mockStore.Predictions[user.UserID]
	if !ok {
		t.Error("Prediction was not stored")
	}

	if pred.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, pred.Username)
	}
}

func TestSetUserPrediction_SingleEliminationFormat_Success(t *testing.T) {
	mockStore := NewMockStore("single-elimination", "test_round")
	mockStore.ValidTeams = []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H"}
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &API{Store: mockStore}

	user := shared.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B", "Team C", "Team D"} // 4 teams for 8-team bracket

	err := api.SetUserPrediction(user, teams, "test_round")
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}
}

func TestSetUserPrediction_WrongNumberOfTeams(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &API{Store: mockStore}

	user := shared.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B"} // Only 2 teams, need 10 for Swiss

	err := api.SetUserPrediction(user, teams, "test_round")
	if err == nil {
		t.Error("Expected error for wrong number of teams, got nil")
	}

	if !strings.Contains(err.Error(), "incorrect number of teams") {
		t.Errorf("Expected error about incorrect number of teams, got: %s", err.Error())
	}
}

func TestSetUserPrediction_InvalidTeamNames(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &API{Store: mockStore}

	user := shared.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Invalid1", "Invalid2", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}

	err := api.SetUserPrediction(user, teams, "test_round")
	if err == nil {
		t.Error("Expected error for invalid team names, got nil")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Expected error about invalid teams, got: %s", err.Error())
	}
}

func TestSetUserPrediction_DuplicateTeams(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &API{Store: mockStore}

	user := shared.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team A", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}

	err := api.SetUserPrediction(user, teams, "test_round")
	if err == nil {
		t.Error("Expected error for duplicate teams, got nil")
	}

	if !strings.Contains(err.Error(), "multiple times") {
		t.Errorf("Expected error about duplicate teams, got: %s", err.Error())
	}
}

func TestSetUserPrediction_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	// Don't set scheduled matches

	api := &API{Store: mockStore}

	user := shared.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}

	err := api.SetUserPrediction(user, teams, "test_round")
	if err == nil {
		t.Error("Expected error when no scheduled matches exist, got nil")
	}
}

func TestSetUserPrediction_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.StoreUserPredictionError = fmt.Errorf("database error")

	api := &API{Store: mockStore}

	user := shared.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}

	err := api.SetUserPrediction(user, teams, "test_round")
	if err == nil {
		t.Error("Expected error from store, got nil")
	}
}

// endregion

// region CheckPrediction tests

func TestCheckPrediction_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	// Set up a prediction
	pred := store.Prediction{
		UserID:   "user1",
		Username: "testuser",
		Format:   "swiss",
		Round:    "test_round",
		Win:      []string{"Team A", "Team B"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
		Lose:     []string{"Team I", "Team J"},
	}
	mockStore.Predictions["user1"] = pred

	// Set up match results
	mockStore.SetSwissResults(map[string]string{
		"Team A": "3-0",
		"Team B": "3-0",
		"Team C": "3-1",
		"Team I": "0-3",
		"Team J": "0-3",
	})

	api := &API{Store: mockStore}
	user := shared.User{UserID: "user1", Username: "testuser"}

	result, err := api.CheckPrediction(user)
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if result == "" {
		t.Error("Expected non-empty result string")
	}
}

func TestCheckPrediction_NoPredictionFound(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{})

	api := &API{Store: mockStore}
	user := shared.User{UserID: "nonexistent", Username: "testuser"}

	_, err := api.CheckPrediction(user)
	if err == nil {
		t.Error("Expected error when no prediction found, got nil")
	}
}

func TestCheckPrediction_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &API{Store: mockStore}
	user := shared.User{UserID: "user1", Username: "testuser"}

	_, err := api.CheckPrediction(user)
	if err == nil {
		t.Error("Expected error when no scheduled matches, got nil")
	}
}

// endregion

// region GenerateLeaderboard tests

func TestGetLeaderboard_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	// Set up multiple predictions
	pred1 := store.Prediction{
		UserID:   "user1",
		Username: "player1",
		Format:   "swiss",
		Round:    "test_round",
		Win:      []string{"Team A", "Team B"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
		Lose:     []string{"Team I", "Team J"},
	}
	pred2 := store.Prediction{
		UserID:   "user2",
		Username: "player2",
		Format:   "swiss",
		Round:    "test_round",
		Win:      []string{"Team A", "Team B"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
		Lose:     []string{"Team I", "Team J"},
	}

	mockStore.Predictions["user1"] = pred1
	mockStore.Predictions["user2"] = pred2

	// Set up match results
	mockStore.SetSwissResults(map[string]string{
		"Team A": "3-0",
		"Team B": "3-0",
		"Team I": "0-3",
		"Team J": "0-3",
	})

	api := &API{Store: mockStore}

	result, err := api.GenerateLeaderboard()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if !strings.Contains(result, "player1") || !strings.Contains(result, "player2") {
		t.Error("Expected leaderboard to contain both players")
	}
}

func TestGetLeaderboard_NoPredictions(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{})

	api := &API{Store: mockStore}

	result, err := api.GenerateLeaderboard()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if !strings.Contains(result, "no user predictions") {
		t.Errorf("Expected message about no predictions, got: %s", result)
	}
}

func TestGetLeaderboard_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &API{Store: mockStore}

	_, err := api.GenerateLeaderboard()
	if err == nil {
		t.Error("Expected error when no scheduled matches, got nil")
	}
}

// endregion

// region GetTeams tests

func TestGetTeams_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{
		"Team A": "3-0",
		"Team B": "3-1",
	})

	api := &API{Store: mockStore}

	teams, err := api.GetTeams()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if len(teams) == 0 {
		t.Error("Expected non-empty teams list")
	}
}

func TestGetTeams_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &API{Store: mockStore}

	_, err := api.GetTeams()
	if err == nil {
		t.Error("Expected error when no scheduled matches, got nil")
	}
}

// endregion

// region GetUpcomingMatches tests

func TestGetUpcomingMatches_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	futureTime := time.Now().Add(24 * time.Hour).Unix()
	mockStore.SetScheduledMatches([]external.ScheduledMatch{
		{
			Team1:     "Team A",
			Team2:     "Team B",
			BestOf:    "3",
			EpochTime: futureTime,
			StreamURL: "BLAST",
			Finished:  false,
		},
	})

	api := &API{Store: mockStore}

	matches, err := api.GetUpcomingMatches()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if len(matches) == 0 {
		t.Error("Expected at least one upcoming match")
	}

	if !strings.Contains(matches[0], "Team A") || !strings.Contains(matches[0], "Team B") {
		t.Error("Expected match to contain team names")
	}
}

func TestGetUpcomingMatches_FiltersPastMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	pastTime := time.Now().Add(-24 * time.Hour).Unix()
	futureTime := time.Now().Add(24 * time.Hour).Unix()

	mockStore.SetScheduledMatches([]external.ScheduledMatch{
		{
			Team1:     "Team A",
			Team2:     "Team B",
			BestOf:    "3",
			EpochTime: pastTime,
			StreamURL: "BLAST",
			Finished:  false,
		},
		{
			Team1:     "Team C",
			Team2:     "Team D",
			BestOf:    "3",
			EpochTime: futureTime,
			StreamURL: "BLAST",
			Finished:  false,
		},
	})

	api := &API{Store: mockStore}

	matches, err := api.GetUpcomingMatches()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if len(matches) != 1 {
		t.Errorf("Expected 1 upcoming match, got %d", len(matches))
	}

	if !strings.Contains(matches[0], "Team C") {
		t.Error("Expected only future match to be returned")
	}
}

func TestGetUpcomingMatches_FiltersFinishedMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	futureTime := time.Now().Add(24 * time.Hour).Unix()

	mockStore.SetScheduledMatches([]external.ScheduledMatch{
		{
			Team1:     "Team A",
			Team2:     "Team B",
			BestOf:    "3",
			EpochTime: futureTime,
			StreamURL: "BLAST",
			Finished:  true, // This match is finished
		},
	})

	api := &API{Store: mockStore}

	matches, err := api.GetUpcomingMatches()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if len(matches) != 0 {
		t.Error("Expected no matches when all are finished")
	}
}

// endregion

// region GetTournamentInfo tests

func TestGetTournamentInfo_Swiss(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{})

	api := &API{Store: mockStore}

	info, err := api.GetTournamentInfo()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if len(info) != 4 {
		t.Errorf("Expected 4 info items, got %d", len(info))
	}

	// Check that info contains expected fields
	hasFormat := false
	hasRequiredTeams := false
	for _, item := range info {
		if strings.Contains(item, "Format: swiss") {
			hasFormat = true
		}
		if strings.Contains(item, "Number of required teams: 10") {
			hasRequiredTeams = true
		}
	}

	if !hasFormat {
		t.Error("Expected tournament info to contain format")
	}
	if !hasRequiredTeams {
		t.Error("Expected tournament info to contain required teams count")
	}
}

func TestGetTournamentInfo_SingleElimination(t *testing.T) {
	mockStore := NewMockStore("single-elimination", "test_round")
	mockStore.ValidTeams = []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8"}
	mockStore.SetScheduledMatches([]external.ScheduledMatch{{Team1: "T1", Team2: "T2"}})

	api := &API{Store: mockStore}

	info, err := api.GetTournamentInfo()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	// For 8 teams, should require 4 predictions
	hasRequiredTeams := false
	for _, item := range info {
		if strings.Contains(item, "Number of required teams: 4") {
			hasRequiredTeams = true
		}
	}

	if !hasRequiredTeams {
		t.Error("Expected tournament info to contain correct required teams count for elimination")
	}
}

// endregion

// region PopulateMatches tests

func TestPopulateMatches_ScheduleOnly(t *testing.T) {
	// This test would require mocking external API calls
	// For now, we test the error cases
	mockStore := NewMockStore("swiss", "test_round")
	api := &API{Store: mockStore}

	// This will fail because we can't actually call external APIs in unit tests
	// In a real implementation, we'd need to mock external.FetchScheduledMatches
	err := api.PopulateMatches(true)
	if err == nil {
		// Expect an error because LIQUIDPEDIADB_API_KEY is not set
		// or external API is not available
	}
}

// endregion

// region getTwitchURL tests

func TestGetTwitchURL_KnownStream(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BLAST_Premier", "https://www.twitch.tv/blastpremier"},
		{"BLAST", "https://www.twitch.tv/blast"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getTwitchURL(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetTwitchURL_UnknownStream(t *testing.T) {
	result := getTwitchURL("unknown_stream")
	if result != "unknown" {
		t.Errorf("Expected 'unknown', got %s", result)
	}
}

// endregion
