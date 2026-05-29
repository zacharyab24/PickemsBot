/* api_test.go
 * Contains unit tests for api.go - testing all public App methods
 * Authors: Zachary Bower
 */

package app

import (
	"fmt"
	"pickems-bot/config"
	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/store"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// region NewApp tests

func TestNewApp_UnsupportedDataSource(t *testing.T) {
	_, err := NewApp(config.Config{DataSource: "unknown", TournamentName: "db", Round: "r1"}, "mongodb://localhost", nil)
	if err == nil {
		t.Error("Expected error for unsupported data source, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported data source") {
		t.Errorf("Expected 'unsupported data source' error, got: %s", err.Error())
	}
}

// endregion

// region SetUserPrediction tests

func TestSetUserPrediction_SwissFormat_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &App{Store: mockStore}

	user := models.User{UserID: "user1", Username: "testuser"}
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
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &App{Store: mockStore}

	user := models.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B", "Team C", "Team D"} // 4 teams for 8-team bracket

	err := api.SetUserPrediction(user, teams, "test_round")
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}
}

func TestSetUserPrediction_WrongNumberOfTeams(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &App{Store: mockStore}

	user := models.User{UserID: "user1", Username: "testuser"}
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
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &App{Store: mockStore}

	user := models.User{UserID: "user1", Username: "testuser"}
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
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &App{Store: mockStore}

	user := models.User{UserID: "user1", Username: "testuser"}
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

	api := &App{Store: mockStore}

	user := models.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}

	err := api.SetUserPrediction(user, teams, "test_round")
	if err == nil {
		t.Error("Expected error when no scheduled matches exist, got nil")
	}
}

func TestSetUserPrediction_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.StoreUserPredictionError = fmt.Errorf("database error")

	api := &App{Store: mockStore}

	user := models.User{UserID: "user1", Username: "testuser"}
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
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	// Set up a prediction
	pred := models.Prediction{
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

	api := &App{Store: mockStore}
	user := models.User{UserID: "user1", Username: "testuser"}

	result, err := api.CheckPrediction(user)
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if result == nil {
		t.Error("Expected non-nil score report")
	}
}

func TestCheckPrediction_NoPredictionFound(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{})

	api := &App{Store: mockStore}
	user := models.User{UserID: "nonexistent", Username: "testuser"}

	_, err := api.CheckPrediction(user)
	if err == nil {
		t.Error("Expected error when no prediction found, got nil")
	}
}

func TestCheckPrediction_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &App{Store: mockStore}
	user := models.User{UserID: "user1", Username: "testuser"}

	_, err := api.CheckPrediction(user)
	if err == nil {
		t.Error("Expected error when no scheduled matches, got nil")
	}
}

// endregion

// region GenerateLeaderboard tests

func TestGenerateLeaderboard_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	// Set up multiple predictions
	pred1 := models.Prediction{
		UserID:   "user1",
		Username: "player1",
		Format:   "swiss",
		Round:    "test_round",
		Win:      []string{"Team A", "Team B"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
		Lose:     []string{"Team I", "Team J"},
	}
	pred2 := models.Prediction{
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

	api := &App{Store: mockStore}

	err := api.GenerateLeaderboard()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	// Verify leaderboard was stored
	if len(mockStore.Leaderboard) != 2 {
		t.Errorf("Expected 2 leaderboard entries, got %d", len(mockStore.Leaderboard))
	}
}

func TestGenerateLeaderboard_NoPredictions(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{})

	api := &App{Store: mockStore}

	err := api.GenerateLeaderboard()
	// When there are no predictions, GetAllUserPredictions returns mongo.ErrNoDocuments
	if err == nil {
		t.Error("Expected error when no predictions exist, got nil")
	}
}

func TestGenerateLeaderboard_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &App{Store: mockStore}

	err := api.GenerateLeaderboard()
	if err == nil {
		t.Error("Expected error when no scheduled matches, got nil")
	}
}

func TestGetLeaderboard_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	// Pre-populate the leaderboard
	mockStore.Leaderboard = []store.LeaderboardEntry{
		{UserID: "user1", Username: "player1", Score: 5, ScoreResult: models.ScoreResult{Successes: 5, Pending: 0, Failed: 0}},
		{UserID: "user2", Username: "player2", Score: 3, ScoreResult: models.ScoreResult{Successes: 3, Pending: 0, Failed: 0}},
	}

	api := &App{Store: mockStore}

	result, err := api.GetLeaderboard()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	found := make(map[string]bool)
	for _, u := range result {
		found[u.Username] = true
	}
	if !found["player1"] || !found["player2"] {
		t.Error("Expected leaderboard to contain both players")
	}
}

func TestGetLeaderboard_NoLeaderboard(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchLeaderboardFromDBError = fmt.Errorf("no leaderboard found")

	api := &App{Store: mockStore}

	_, err := api.GetLeaderboard()
	if err == nil {
		t.Error("Expected error when no leaderboard exists, got nil")
	}
}

// endregion

// region GetTeams tests

func TestGetTeams_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{
		"Team A": "3-0",
		"Team B": "3-1",
	})

	api := &App{Store: mockStore}

	teams, err := api.GetTeams()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if len(teams) == 0 {
		t.Error("Expected non-empty teams list")
	}
}

func TestGetTeams_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetValidTeamsError = fmt.Errorf("database error")

	api := &App{Store: mockStore}

	_, err := api.GetTeams()
	if err == nil {
		t.Error("Expected error when store returns an error, got nil")
	}
}

// endregion

// region normalizeForVRSLookup tests

func TestNormalizeForVRSLookup(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Team Liquid", "liquid"},
		{"BetBoom Team", "betboom"},
		{"Sharks Esports", "sharks"},
		{"Lynn Vision Gaming", "lynn vision"},
		{"HEROIC", "heroic"},
		{"The MongolZ", "the mongolz"},
		{"FaZe", "faze"},
		{"", ""},
		{"team liquid", "liquid"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeForVRSLookup(tc.input)
			if got != tc.want {
				t.Errorf("normalizeForVRSLookup(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// endregion

// region GetTeam tests

func TestGetTeam_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "FaZe", Standing: 5, Points: 1750, Roster: []string{"karrigan", "broky", "rain"}},
	})
	api := &App{Store: mockStore}

	entry, err := api.GetTeam("FaZe")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if entry.Standing != 5 {
		t.Errorf("expected standing 5, got %d", entry.Standing)
	}
}

func TestGetTeam_NormalisationMatch(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "Liquid", Standing: 47, Points: 1164},
	})
	api := &App{Store: mockStore}

	entry, err := api.GetTeam("Team Liquid")
	if err != nil {
		t.Fatalf("expected no error for normalised match, got: %v", err)
	}
	if entry.Standing != 47 {
		t.Errorf("expected standing 47, got %d", entry.Standing)
	}
}

func TestGetTeam_EmptyName(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := &App{Store: mockStore}

	_, err := api.GetTeam("")
	if err == nil {
		t.Error("expected error for empty team name, got nil")
	}
}

func TestGetTeam_NotFound(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "FaZe", Standing: 5},
	})
	api := &App{Store: mockStore}

	_, err := api.GetTeam("Unknown Team")
	if err == nil {
		t.Error("expected error for unknown team, got nil")
	}
}

func TestGetTeam_FuzzyMatch(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "THUNDER dOWNUNDER", Standing: 22},
	})
	api := &App{Store: mockStore}

	// "THUNDERdOWNUNDER" vs "THUNDER dOWNUNDER" — spacing difference, should fuzzy match
	entry, err := api.GetTeam("THUNDERdOWNUNDER")
	if err != nil {
		t.Fatalf("expected fuzzy match to succeed, got: %v", err)
	}
	if entry.Standing != 22 {
		t.Errorf("expected standing 22, got %d", entry.Standing)
	}
}

func TestGetTeam_FetchError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchVrsDataFromDBError = fmt.Errorf("db connection failed")
	api := &App{Store: mockStore}

	_, err := api.GetTeam("FaZe")
	if err == nil {
		t.Error("expected error when VRS fetch fails, got nil")
	}
}

// endregion

// region GetTeams VRS tests

func TestGetTeams_WithVRSData(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ValidTeams = []string{"Team Liquid", "FaZe"}
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "Liquid", Standing: 47},
		{TeamName: "FaZe", Standing: 5},
	})
	api := &App{Store: mockStore}

	teams, err := api.GetTeams()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	rankings := make(map[string]int)
	for _, tm := range teams {
		rankings[tm.Name] = tm.VRSRanking
	}
	if rankings["Team Liquid"] != 47 {
		t.Errorf("expected Team Liquid ranking 47, got %d", rankings["Team Liquid"])
	}
	if rankings["FaZe"] != 5 {
		t.Errorf("expected FaZe ranking 5, got %d", rankings["FaZe"])
	}
}

func TestGetTeams_UnmatchedTeamGetsZero(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ValidTeams = []string{"CompletelyUnknownOrg"}
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "FaZe", Standing: 5},
	})
	api := &App{Store: mockStore}

	teams, err := api.GetTeams()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(teams) == 0 {
		t.Fatal("expected at least one team")
	}
	if teams[0].VRSRanking != 0 {
		t.Errorf("expected unmatched team to have ranking 0, got %d", teams[0].VRSRanking)
	}
}

func TestGetTeams_VRSFetchError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetSwissResults(map[string]string{"Team A": "1-0"})
	mockStore.FetchVrsDataFromDBError = fmt.Errorf("db error")
	api := &App{Store: mockStore}

	_, err := api.GetTeams()
	if err == nil {
		t.Error("expected error when VRS fetch fails, got nil")
	}
}

// endregion

// region GetUpcomingMatches tests

func TestGetUpcomingMatches_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	futureTime := time.Now().Add(24 * time.Hour).Unix()
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{
			Team1:     "Team A",
			Team2:     "Team B",
			BestOf:    "3",
			EpochTime: futureTime,
			StreamURL: "BLAST",
			Finished:  false,
		},
	})

	api := &App{Store: mockStore}

	matches, err := api.GetUpcomingMatches()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if len(matches) == 0 {
		t.Error("Expected at least one upcoming match")
	}

	if matches[0].Team1 != "Team A" || matches[0].Team2 != "Team B" {
		t.Error("Expected match to contain team names")
	}
}

func TestGetUpcomingMatches_FiltersPastMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	pastTime := time.Now().Add(-24 * time.Hour).Unix()
	futureTime := time.Now().Add(24 * time.Hour).Unix()

	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
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

	api := &App{Store: mockStore}

	matches, err := api.GetUpcomingMatches()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if len(matches) != 1 {
		t.Errorf("Expected 1 upcoming match, got %d", len(matches))
	}

	if matches[0].Team1 != "Team C" && matches[0].Team2 != "Team C" {
		t.Error("Expected only future match to be returned")
	}
}

func TestGetUpcomingMatches_FiltersFinishedMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	futureTime := time.Now().Add(24 * time.Hour).Unix()

	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{
			Team1:     "Team A",
			Team2:     "Team B",
			BestOf:    "3",
			EpochTime: futureTime,
			StreamURL: "BLAST",
			Finished:  true, // This match is finished
		},
	})

	api := &App{Store: mockStore}

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
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{})

	api := &App{Store: mockStore}

	info, err := api.GetTournamentInfo()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if info.Format != "swiss" {
		t.Errorf("Expected format 'swiss', got %q", info.Format)
	}
	if info.NumTeams != 10 {
		t.Errorf("Expected 10 required teams, got %d", info.NumTeams)
	}
}

func TestGetTournamentInfo_SingleElimination(t *testing.T) {
	mockStore := NewMockStore("single-elimination", "test_round")
	mockStore.ValidTeams = []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8"}
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "T1", Team2: "T2"}})

	api := &App{Store: mockStore}

	info, err := api.GetTournamentInfo()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	// For 8 teams, should require 4 predictions
	if info.NumTeams != 4 {
		t.Errorf("Expected 4 required teams for 8-team elimination, got %d", info.NumTeams)
	}
}

func TestGetTournamentInfo_GetValidTeamsError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.GetValidTeamsError = fmt.Errorf("db error")

	api := &App{Store: mockStore}

	_, err := api.GetTournamentInfo()
	if err == nil {
		t.Error("expected error when GetValidTeams fails, got nil")
	}
}

func TestGetTournamentInfo_UnknownFormat(t *testing.T) {
	mockStore := NewMockStore("unknown-format", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &App{Store: mockStore}

	_, err := api.GetTournamentInfo()
	if err == nil {
		t.Error("expected error for unknown tournament format, got nil")
	}
}

// endregion

// region PopulateMatches tests

func TestPopulateMatches_ScheduleError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchAndStoreScheduleError = fmt.Errorf("schedule fetch failed")

	api := &App{
		Store:       mockStore,
		rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10),
	}

	err := api.PopulateMatches(true)
	if err == nil {
		t.Error("expected error when schedule fetch fails, got nil")
	}
}

func TestPopulateMatches_ResultsError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchAndUpdateMatchResultsError = fmt.Errorf("results fetch failed")

	api := &App{
		Store:       mockStore,
		rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10),
	}

	err := api.PopulateMatches(false)
	if err == nil {
		t.Error("expected error when results fetch fails, got nil")
	}
}

// endregion

// region UpdateMatchResults tests

func TestUpdateMatchResults_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})

	api := &App{
		Store:       mockStore,
		rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10),
	}

	err := api.UpdateMatchResults()
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}
}

func TestUpdateMatchResults_RateLimiterNil(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &App{
		Store:       mockStore,
		rateLimiter: nil,
	}

	err := api.UpdateMatchResults()
	if err == nil {
		t.Error("Expected error when rate limiter is nil, got nil")
	}

	if !strings.Contains(err.Error(), "rate limiter") {
		t.Errorf("Expected rate limiter error, got: %s", err.Error())
	}
}

func TestUpdateMatchResults_RateLimitExceeded(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	// Create a rate limiter that's already exhausted
	limiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	limiter.Allow() // exhaust the single token

	api := &App{
		Store:       mockStore,
		rateLimiter: limiter,
	}

	err := api.UpdateMatchResults()
	if err == nil {
		t.Error("Expected error when rate limit exceeded, got nil")
	}

	if !strings.Contains(err.Error(), "rate limiter exceeded") {
		t.Errorf("Expected rate limit error, got: %s", err.Error())
	}
}

func TestUpdateMatchResults_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchAndUpdateMatchResultsError = fmt.Errorf("store error")

	api := &App{
		Store:       mockStore,
		rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10),
	}

	err := api.UpdateMatchResults()
	if err == nil {
		t.Error("Expected error from store, got nil")
	}

	if !strings.Contains(err.Error(), "store error") {
		t.Errorf("Expected store error, got: %s", err.Error())
	}
}

// endregion

// region logger tests

func TestApp_Logger_NilLogger_ReturnsDefault(t *testing.T) {
	a := &App{}
	// logger() must not panic and must return slog.Default()
	l := a.logger()
	if l == nil {
		t.Error("Expected non-nil logger when no logger injected")
	}
}

// endregion

// region PopulateMatches success paths

func TestPopulateMatches_ScheduleOnly_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &App{
		Store:       mockStore,
		rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10),
	}

	err := api.PopulateMatches(true)
	if err != nil {
		t.Errorf("Expected no error for scheduleOnly=true, got: %s", err.Error())
	}
}

func TestPopulateMatches_FullUpdate_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &App{
		Store:       mockStore,
		rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10),
	}

	err := api.PopulateMatches(false)
	if err != nil {
		t.Errorf("Expected no error for scheduleOnly=false, got: %s", err.Error())
	}
}

// endregion

// region PopulateMatches rate limiter tests

func TestPopulateMatches_RateLimiterNil(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	api := &App{
		Store:       mockStore,
		rateLimiter: nil,
	}

	err := api.PopulateMatches(true)
	if err == nil {
		t.Error("Expected error when rate limiter is nil, got nil")
	}

	if !strings.Contains(err.Error(), "rate limiter") {
		t.Errorf("Expected rate limiter error, got: %s", err.Error())
	}
}

func TestPopulateMatches_RateLimitExceeded(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")

	// Create a rate limiter that's already exhausted
	limiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	limiter.Allow() // exhaust the single token

	api := &App{
		Store:       mockStore,
		rateLimiter: limiter,
	}

	err := api.PopulateMatches(true)
	if err == nil {
		t.Error("Expected error when rate limit exceeded, got nil")
	}

	if !strings.Contains(err.Error(), "rate limiter limit reached") {
		t.Errorf("Expected rate limit error, got: %s", err.Error())
	}
}

// endregion
