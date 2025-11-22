package store

import (
	"context"
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
)

func TestStoreUserPrediction_Insert(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	userID := "user123"
	prediction := Prediction{
		UserID:   userID,
		Username: "testuser",
		Format:   "swiss",
		Round:    "test-round",
		Win:      []string{"Team A", "Team B"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F"},
		Lose:     []string{"Team G", "Team H"},
	}

	err := store.StoreUserPrediction(userID, prediction)
	if err != nil {
		t.Fatalf("Failed to store prediction: %v", err)
	}

	// Verify it was stored
	retrieved, err := store.GetUserPrediction(userID)
	if err != nil {
		t.Fatalf("Failed to retrieve prediction: %v", err)
	}

	if retrieved.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, retrieved.UserID)
	}
	if retrieved.Username != "testuser" {
		t.Errorf("Expected Username testuser, got %s", retrieved.Username)
	}
	if len(retrieved.Win) != 2 {
		t.Errorf("Expected 2 win teams, got %d", len(retrieved.Win))
	}
}

func TestStoreUserPrediction_Update(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	userID := "user456"

	// Insert initial prediction
	original := Prediction{
		UserID:   userID,
		Username: "testuser",
		Format:   "swiss",
		Round:    "test-round",
		Win:      []string{"Team A", "Team B"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F"},
		Lose:     []string{"Team G", "Team H"},
	}

	err := store.StoreUserPrediction(userID, original)
	if err != nil {
		t.Fatalf("Failed to store original prediction: %v", err)
	}

	// Update with new prediction
	updated := Prediction{
		UserID:   userID,
		Username: "testuser",
		Format:   "swiss",
		Round:    "test-round",
		Win:      []string{"Team X", "Team Y"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F"},
		Lose:     []string{"Team G", "Team H"},
	}

	err = store.StoreUserPrediction(userID, updated)
	if err != nil {
		t.Fatalf("Failed to update prediction: %v", err)
	}

	// Verify it was updated
	retrieved, err := store.GetUserPrediction(userID)
	if err != nil {
		t.Fatalf("Failed to retrieve prediction: %v", err)
	}

	if len(retrieved.Win) != 2 {
		t.Errorf("Expected 2 win teams, got %d", len(retrieved.Win))
	}
	if retrieved.Win[0] != "Team X" {
		t.Errorf("Expected first win team to be Team X, got %s", retrieved.Win[0])
	}
}

func TestStoreUserPrediction_Elimination(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	userID := "user789"
	prediction := Prediction{
		UserID:   userID,
		Username: "testuser",
		Format:   "single-elimination",
		Round:    "test-round",
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "grandfinal", Status: "advanced"},
			"Team B": {Round: "semifinal", Status: "eliminated"},
		},
	}

	err := store.StoreUserPrediction(userID, prediction)
	if err != nil {
		t.Fatalf("Failed to store elimination prediction: %v", err)
	}

	// Verify it was stored
	retrieved, err := store.GetUserPrediction(userID)
	if err != nil {
		t.Fatalf("Failed to retrieve prediction: %v", err)
	}

	if retrieved.Format != "single-elimination" {
		t.Errorf("Expected format single-elimination, got %s", retrieved.Format)
	}
	if len(retrieved.Progression) != 2 {
		t.Errorf("Expected 2 teams in progression, got %d", len(retrieved.Progression))
	}
}

func TestGetUserPrediction_NotFound(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	_, err := store.GetUserPrediction("nonexistent")
	if err != mongo.ErrNoDocuments {
		t.Errorf("Expected ErrNoDocuments, got %v", err)
	}
}

func TestGetAllUserPredictions_Empty(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	predictions, err := store.GetAllUserPredictions()
	if err != nil {
		t.Fatalf("Failed to get all predictions: %v", err)
	}

	if len(predictions) != 0 {
		t.Errorf("Expected 0 predictions, got %d", len(predictions))
	}
}

func TestGetAllUserPredictions_Multiple(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Insert multiple predictions
	users := []string{"user1", "user2", "user3"}
	for _, userID := range users {
		prediction := Prediction{
			UserID:   userID,
			Username: "test" + userID,
			Format:   "swiss",
			Round:    "test-round",
			Win:      []string{"Team A", "Team B"},
			Advance:  []string{"Team C", "Team D", "Team E", "Team F"},
			Lose:     []string{"Team G", "Team H"},
		}
		err := store.StoreUserPrediction(userID, prediction)
		if err != nil {
			t.Fatalf("Failed to store prediction for %s: %v", userID, err)
		}
	}

	// Retrieve all
	predictions, err := store.GetAllUserPredictions()
	if err != nil {
		t.Fatalf("Failed to get all predictions: %v", err)
	}

	if len(predictions) != 3 {
		t.Errorf("Expected 3 predictions, got %d", len(predictions))
	}

	// Verify all userIDs are present
	userIDMap := make(map[string]bool)
	for _, pred := range predictions {
		userIDMap[pred.UserID] = true
	}
	for _, userID := range users {
		if !userIDMap[userID] {
			t.Errorf("Expected to find userID %s in results", userID)
		}
	}
}

func TestGetValidTeams_NoMatchResults(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	_, _, err := store.GetValidTeams()
	if err == nil {
		t.Fatal("Expected error when no match results in DB, got nil")
	}
}

func TestGetValidTeams_Swiss(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Store swiss match results
	results := external.SwissResult{
		Scores: map[string]string{
			"Team A": "3-0",
			"Team B": "3-1",
			"Team C": "2-3",
			"Team D": "0-3",
		},
	}

	upcomingMatches := []external.ScheduledMatch{}
	err := store.StoreMatchResults(results, upcomingMatches)
	if err != nil {
		t.Fatalf("Failed to store match results: %v", err)
	}

	teams, format, err := store.GetValidTeams()
	if err != nil {
		t.Fatalf("Failed to get valid teams: %v", err)
	}

	if format != "swiss" {
		t.Errorf("Expected swiss format, got %s", format)
	}

	expectedTeams := []string{"Team A", "Team B", "Team C", "Team D"}
	if len(teams) != len(expectedTeams) {
		t.Errorf("Expected %d teams, got %d", len(expectedTeams), len(teams))
	}

	// Verify no duplicates and all teams present
	teamSet := make(map[string]bool)
	for _, team := range teams {
		if teamSet[team] {
			t.Errorf("Duplicate team found: %s", team)
		}
		teamSet[team] = true
	}

	for _, expected := range expectedTeams {
		if !teamSet[expected] {
			t.Errorf("Expected team %s not found", expected)
		}
	}
}

func TestGetValidTeams_Elimination(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Store elimination results
	results := external.EliminationResult{
		Progression: map[string]shared.TeamProgress{
			"Team A": {Round: "grandfinal", Status: "advanced"},
			"Team B": {Round: "semifinal", Status: "eliminated"},
		},
	}

	upcomingMatches := []external.ScheduledMatch{}
	err := store.StoreMatchResults(results, upcomingMatches)
	if err != nil {
		t.Fatalf("Failed to store match results: %v", err)
	}

	teams, format, err := store.GetValidTeams()
	if err != nil {
		t.Fatalf("Failed to get valid teams: %v", err)
	}

	if format != "single-elimination" {
		t.Errorf("Expected single-elimination format, got %s", format)
	}

	if len(teams) != 2 {
		t.Errorf("Expected 2 teams, got %d", len(teams))
	}
}

func TestGetValidTeams_OnlyRealTeams(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Store swiss results with only real teams (GetValidTeams doesn't filter TBD,
	// it just returns what's in the results table)
	results := external.SwissResult{
		Scores: map[string]string{
			"Team A": "3-0",
			"Team B": "2-1",
			"Team C": "1-2",
			"Team D": "0-3",
		},
	}

	upcomingMatches := []external.ScheduledMatch{}
	err := store.StoreMatchResults(results, upcomingMatches)
	if err != nil {
		t.Fatalf("Failed to store match results: %v", err)
	}

	teams, _, err := store.GetValidTeams()
	if err != nil {
		t.Fatalf("Failed to get valid teams: %v", err)
	}

	expectedTeams := []string{"Team A", "Team B", "Team C", "Team D"}
	if len(teams) != len(expectedTeams) {
		t.Errorf("Expected %d teams, got %d", len(expectedTeams), len(teams))
	}

	// Verify all expected teams are present
	teamSet := make(map[string]bool)
	for _, team := range teams {
		teamSet[team] = true
	}
	for _, expected := range expectedTeams {
		if !teamSet[expected] {
			t.Errorf("Expected team %s not found", expected)
		}
	}
}
