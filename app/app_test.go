/* app_test.go
 * Unit tests for App public methods.
 * Authors: Zachary Bower
 */

package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/store"

	"golang.org/x/time/rate"
)

const (
	testGuildID   = "test_guild"
	testChannelID = ""
)

// bg is shorthand for context.Background() in tests.
func bg() context.Context { return context.Background() }

// region resolveConfig

func TestResolveConfig_NoGuildConfig(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetGuildConfigError = fmt.Errorf("not found")
	api := NewTestApp(mockStore)

	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "u1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "no configuration found") {
		t.Errorf("expected 'no configuration found' error, got: %v", err)
	}
}

func TestResolveConfig_NilTournamentID(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GuildConfig.TournamentID = nil
	api := NewTestApp(mockStore)

	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "u1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "tournament not configured") {
		t.Errorf("expected 'tournament not configured' error, got: %v", err)
	}
}

func TestResolveConfig_NilRound(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GuildConfig.Round = nil
	api := NewTestApp(mockStore)

	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "u1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "tournament not configured") {
		t.Errorf("expected 'tournament not configured' error, got: %v", err)
	}
}

// endregion

// region SetUserPrediction

func TestSetUserPrediction_SwissFormat_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := NewTestApp(mockStore)

	user := models.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}

	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, user, teams)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	pred, ok := mockStore.Predictions[user.UserID]
	if !ok {
		t.Fatal("prediction was not stored")
	}
	if pred.Username != user.Username {
		t.Errorf("expected username %s, got %s", user.Username, pred.Username)
	}
}

func TestSetUserPrediction_SingleEliminationFormat_Success(t *testing.T) {
	mockStore := NewMockStore("single-elimination", "test_round")
	mockStore.ValidTeams = []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H"}
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := NewTestApp(mockStore)

	user := models.User{UserID: "user1", Username: "testuser"}
	teams := []string{"Team A", "Team B", "Team C", "Team D"}

	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, user, teams)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestSetUserPrediction_WrongNumberOfTeams(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := NewTestApp(mockStore)

	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID,
		models.User{UserID: "u1"}, []string{"Team A", "Team B"})
	if err == nil || !strings.Contains(err.Error(), "incorrect number of teams") {
		t.Errorf("expected 'incorrect number of teams' error, got: %v", err)
	}
}

func TestSetUserPrediction_InvalidTeamNames(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := NewTestApp(mockStore)

	teams := []string{"Invalid1", "Invalid2", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}
	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "u1"}, teams)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected 'invalid' error, got: %v", err)
	}
}

func TestSetUserPrediction_DuplicateTeams(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := NewTestApp(mockStore)

	teams := []string{"Team A", "Team A", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}
	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "u1"}, teams)
	if err == nil || !strings.Contains(err.Error(), "multiple times") {
		t.Errorf("expected 'multiple times' error, got: %v", err)
	}
}

func TestSetUserPrediction_FuzzyCollision(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := NewTestApp(mockStore)

	// "Team A" and "team a" both fuzzy-resolve to "Team A"
	teams := []string{"Team A", "team a", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}
	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "u1"}, teams)
	if err == nil || !strings.Contains(err.Error(), "both resolved to") {
		t.Errorf("expected 'both resolved to' error, got: %v", err)
	}
}

func TestSetUserPrediction_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	// no matches set — EnsureScheduledMatches returns error
	api := NewTestApp(mockStore)

	teams := []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}
	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "u1"}, teams)
	if err == nil {
		t.Error("expected error when no scheduled matches exist, got nil")
	}
}

func TestSetUserPrediction_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.UpsertPredictionError = fmt.Errorf("database error")
	api := NewTestApp(mockStore)

	teams := []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J"}
	_, err := api.SetUserPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "u1"}, teams)
	if err == nil {
		t.Error("expected error from store, got nil")
	}
}

// endregion

// region CheckPrediction

func TestCheckPrediction_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.Predictions["user1"] = models.Prediction{
		UserID: "user1", Username: "testuser", Format: "swiss", Round: "test_round",
		Win:     []string{"Team A", "Team B"},
		Advance: []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
		Lose:    []string{"Team I", "Team J"},
	}
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0", "Team B": "3-0", "Team I": "0-3", "Team J": "0-3"})
	api := NewTestApp(mockStore)

	result, err := api.CheckPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "user1"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil score report")
	}
}

func TestCheckPrediction_NoPredictionFound(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{})
	api := NewTestApp(mockStore)

	_, err := api.CheckPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "nonexistent"})
	if err == nil {
		t.Error("expected error when no prediction found, got nil")
	}
}

func TestCheckPrediction_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := NewTestApp(mockStore)

	_, err := api.CheckPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "user1"})
	if err == nil {
		t.Error("expected error when no scheduled matches, got nil")
	}
}

func TestCheckPrediction_GetMatchResultsError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.Predictions["user1"] = models.Prediction{UserID: "user1", Format: "swiss", Round: "test_round"}
	mockStore.GetMatchResultsError = fmt.Errorf("results error")
	api := NewTestApp(mockStore)

	_, err := api.CheckPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "user1"})
	if err == nil || !strings.Contains(err.Error(), "results error") {
		t.Errorf("expected match results error, got: %v", err)
	}
}

func TestCheckPrediction_NoGuildConfig(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetGuildConfigError = fmt.Errorf("not found")
	api := NewTestApp(mockStore)

	_, err := api.CheckPrediction(bg(), testGuildID, testChannelID, models.User{UserID: "user1"})
	if err == nil {
		t.Error("expected error when no guild config, got nil")
	}
}

// endregion

// region CheckPredictionByUsername

func TestCheckPredictionByUsername_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0", "Team B": "3-0", "Team I": "0-3", "Team J": "0-3"})
	mockStore.Predictions["user1"] = models.Prediction{
		UserID: "user1", Username: "PickemsBot", Format: "swiss", Round: "test_round",
		Win:     []string{"Team A", "Team B"},
		Advance: []string{"Team C", "Team D", "Team E", "Team F", "Team G", "Team H"},
		Lose:    []string{"Team I", "Team J"},
	}
	api := NewTestApp(mockStore)

	user, report, err := api.CheckPredictionByUsername(bg(), testGuildID, testChannelID, "pickemsbot")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if report == nil {
		t.Error("expected non-nil score report")
	}
	if user.Username != "PickemsBot" {
		t.Errorf("expected canonical username PickemsBot, got %s", user.Username)
	}
}

func TestCheckPredictionByUsername_NotFound(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{})
	api := NewTestApp(mockStore)

	_, _, err := api.CheckPredictionByUsername(bg(), testGuildID, testChannelID, "ghost")
	if err == nil {
		t.Error("expected error when username not found, got nil")
	}
}

func TestCheckPredictionByUsername_NoScheduledMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := NewTestApp(mockStore)

	_, _, err := api.CheckPredictionByUsername(bg(), testGuildID, testChannelID, "PickemsBot")
	if err == nil {
		t.Error("expected error when no scheduled matches, got nil")
	}
}

func TestCheckPredictionByUsername_NoGuildConfig(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetGuildConfigError = fmt.Errorf("not found")
	api := NewTestApp(mockStore)

	_, _, err := api.CheckPredictionByUsername(bg(), testGuildID, testChannelID, "PickemsBot")
	if err == nil {
		t.Error("expected error when no guild config, got nil")
	}
}

// endregion

// region GetLeaderboard

func TestGetLeaderboard_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Leaderboard = []store.LeaderboardEntry{
		{UserID: "user1", Username: "player1", Successes: 5},
		{UserID: "user2", Username: "player2", Successes: 3},
	}
	api := NewTestApp(mockStore)

	result, err := api.GetLeaderboard(bg(), testGuildID, testChannelID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	found := make(map[string]bool)
	for _, u := range result {
		found[u.Username] = true
	}
	if !found["player1"] || !found["player2"] {
		t.Error("expected leaderboard to contain both players")
	}
}

func TestGetLeaderboard_SortedBySuccesses(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Leaderboard = []store.LeaderboardEntry{
		{UserID: "u2", Username: "player2", Successes: 3},
		{UserID: "u1", Username: "player1", Successes: 7},
	}
	api := NewTestApp(mockStore)

	result, err := api.GetLeaderboard(bg(), testGuildID, testChannelID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result[0].Username != "player1" {
		t.Errorf("expected player1 first (7 successes), got %s", result[0].Username)
	}
	if result[0].Rank != 1 || result[1].Rank != 2 {
		t.Errorf("expected ranks 1,2 got %d,%d", result[0].Rank, result[1].Rank)
	}
}

func TestGetLeaderboard_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetLeaderboardError = fmt.Errorf("no leaderboard found")
	api := NewTestApp(mockStore)

	_, err := api.GetLeaderboard(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error when store returns an error, got nil")
	}
}

func TestGetLeaderboard_NoGuildConfig(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetGuildConfigError = fmt.Errorf("not found")
	api := NewTestApp(mockStore)

	_, err := api.GetLeaderboard(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error when no guild config, got nil")
	}
}

// endregion

// region GetTeams

func TestGetTeams_WithVRSData(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ValidTeams = []string{"Team Liquid", "FaZe"}
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "Liquid", Standing: 47},
		{TeamName: "FaZe", Standing: 5},
	})
	api := NewTestApp(mockStore)

	teams, err := api.GetTeams(bg(), testGuildID, testChannelID)
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
	mockStore.SetVRSEntries([]store.VRSEntry{{TeamName: "FaZe", Standing: 5}})
	api := NewTestApp(mockStore)

	teams, err := api.GetTeams(bg(), testGuildID, testChannelID)
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

func TestGetTeams_ListValidTeamsError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ListValidTeamsError = fmt.Errorf("database error")
	api := NewTestApp(mockStore)

	_, err := api.GetTeams(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error when ListValidTeams fails, got nil")
	}
}

func TestGetTeams_VRSFetchError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ListVRSRankingsError = fmt.Errorf("db error")
	api := NewTestApp(mockStore)

	_, err := api.GetTeams(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error when VRS fetch fails, got nil")
	}
}

func TestGetTeams_NoGuildConfig(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetGuildConfigError = fmt.Errorf("not found")
	api := NewTestApp(mockStore)

	_, err := api.GetTeams(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error when no guild config, got nil")
	}
}

// endregion

// region GetTeam

func TestGetTeam_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "FaZe", Standing: 5, Points: 1750, Roster: []string{"karrigan", "broky", "rain"}},
	})
	api := NewTestApp(mockStore)

	entry, err := api.GetTeam(bg(), "FaZe")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if entry.Standing != 5 {
		t.Errorf("expected standing 5, got %d", entry.Standing)
	}
}

func TestGetTeam_NormalisationMatch(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{{TeamName: "Liquid", Standing: 47, Points: 1164}})
	api := NewTestApp(mockStore)

	entry, err := api.GetTeam(bg(), "Team Liquid")
	if err != nil {
		t.Fatalf("expected normalised match to succeed, got: %v", err)
	}
	if entry.Standing != 47 {
		t.Errorf("expected standing 47, got %d", entry.Standing)
	}
}

func TestGetTeam_EmptyName(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := NewTestApp(mockStore)

	_, err := api.GetTeam(bg(), "")
	if err == nil {
		t.Error("expected error for empty team name, got nil")
	}
}

func TestGetTeam_NotFound(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{{TeamName: "FaZe", Standing: 5}})
	api := NewTestApp(mockStore)

	_, err := api.GetTeam(bg(), "Unknown Team")
	if err == nil {
		t.Error("expected error for unknown team, got nil")
	}
}

func TestGetTeam_FuzzyMatch(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{{TeamName: "THUNDER dOWNUNDER", Standing: 22}})
	api := NewTestApp(mockStore)

	entry, err := api.GetTeam(bg(), "THUNDERdOWNUNDER")
	if err != nil {
		t.Fatalf("expected fuzzy match to succeed, got: %v", err)
	}
	if entry.Standing != 22 {
		t.Errorf("expected standing 22, got %d", entry.Standing)
	}
}

func TestGetTeam_FetchError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ListVRSRankingsError = fmt.Errorf("db connection failed")
	api := NewTestApp(mockStore)

	_, err := api.GetTeam(bg(), "FaZe")
	if err == nil {
		t.Error("expected error when VRS fetch fails, got nil")
	}
}

// endregion

// region GetUpcomingMatches

func TestGetUpcomingMatches_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3", EpochTime: time.Now().Add(24 * time.Hour).Unix(), Finished: false},
	})
	api := NewTestApp(mockStore)

	matches, err := api.GetUpcomingMatches(bg(), testGuildID, testChannelID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(matches) == 0 {
		t.Error("expected at least one upcoming match")
	}
	if matches[0].Team1 != "Team A" || matches[0].Team2 != "Team B" {
		t.Error("unexpected match teams")
	}
}

func TestGetUpcomingMatches_LiveMatchIncluded(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: time.Now().Add(-24 * time.Hour).Unix(), Finished: false},
		{Team1: "Team C", Team2: "Team D", EpochTime: time.Now().Add(24 * time.Hour).Unix(), Finished: false},
	})
	api := NewTestApp(mockStore)

	matches, err := api.GetUpcomingMatches(bg(), testGuildID, testChannelID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches (1 live + 1 upcoming), got %d", len(matches))
	}
	if !matches[0].Live {
		t.Error("expected past unfinished match to be marked as live")
	}
	if matches[1].Live {
		t.Error("expected future match to not be marked as live")
	}
}

func TestGetUpcomingMatches_FiltersFinishedMatches(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: time.Now().Add(24 * time.Hour).Unix(), Finished: true},
	})
	api := NewTestApp(mockStore)

	matches, err := api.GetUpcomingMatches(bg(), testGuildID, testChannelID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no matches when all are finished, got %d", len(matches))
	}
}

func TestGetUpcomingMatches_DisplaysInChronologicalOrder(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team C", Team2: "Team D", EpochTime: time.Now().Add(25 * time.Hour).Unix(), Finished: false},
		{Team1: "Team A", Team2: "Team B", EpochTime: time.Now().Add(24 * time.Hour).Unix(), Finished: false},
	})
	api := NewTestApp(mockStore)

	matches, err := api.GetUpcomingMatches(bg(), testGuildID, testChannelID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].EpochTime > matches[1].EpochTime {
		t.Error("expected matches to be sorted chronologically")
	}
}

func TestGetUpcomingMatches_GetScheduleError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.GetMatchScheduleError = fmt.Errorf("fetch failed")
	api := NewTestApp(mockStore)

	_, err := api.GetUpcomingMatches(bg(), testGuildID, testChannelID)
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Errorf("expected fetch error, got: %v", err)
	}
}

func TestGetUpcomingMatches_NoGuildConfig(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetGuildConfigError = fmt.Errorf("not found")
	api := NewTestApp(mockStore)

	_, err := api.GetUpcomingMatches(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error when no guild config, got nil")
	}
}

// endregion

// region GetTournamentInfo

func TestGetTournamentInfo_Swiss(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := NewTestApp(mockStore)

	info, err := api.GetTournamentInfo(bg(), testGuildID, testChannelID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if info.Format != "swiss" {
		t.Errorf("expected format 'swiss', got %q", info.Format)
	}
	if info.NumTeams != 10 {
		t.Errorf("expected 10 required teams, got %d", info.NumTeams)
	}
}

func TestGetTournamentInfo_SingleElimination(t *testing.T) {
	mockStore := NewMockStore("single-elimination", "test_round")
	mockStore.ValidTeams = []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8"}
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "T1", Team2: "T2"}})
	api := NewTestApp(mockStore)

	info, err := api.GetTournamentInfo(bg(), testGuildID, testChannelID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if info.NumTeams != 4 {
		t.Errorf("expected 4 required teams for 8-team bracket, got %d", info.NumTeams)
	}
}

func TestGetTournamentInfo_ListValidTeamsError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.ListValidTeamsError = fmt.Errorf("db error")
	api := NewTestApp(mockStore)

	_, err := api.GetTournamentInfo(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error when ListValidTeams fails, got nil")
	}
}

func TestGetTournamentInfo_UnknownFormat(t *testing.T) {
	mockStore := NewMockStore("unknown-format", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := NewTestApp(mockStore)

	_, err := api.GetTournamentInfo(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error for unknown tournament format, got nil")
	}
}

func TestGetTournamentInfo_NoGuildConfig(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetGuildConfigError = fmt.Errorf("not found")
	api := NewTestApp(mockStore)

	_, err := api.GetTournamentInfo(bg(), testGuildID, testChannelID)
	if err == nil {
		t.Error("expected error when no guild config, got nil")
	}
}

// endregion

// region PopulateMatches

func TestPopulateMatches_ScheduleOnly_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := &App{Store: mockStore, rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10)}

	if err := api.PopulateMatches(bg(), 1, "test_round", true); err != nil {
		t.Errorf("expected no error for scheduleOnly=true, got: %v", err)
	}
}

func TestPopulateMatches_FullUpdate_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := &App{Store: mockStore, rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10)}

	if err := api.PopulateMatches(bg(), 1, "test_round", false); err != nil {
		t.Errorf("expected no error for scheduleOnly=false, got: %v", err)
	}
}

func TestPopulateMatches_ScheduleError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchAndSaveScheduleError = fmt.Errorf("schedule fetch failed")
	api := &App{Store: mockStore, rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10)}

	// Schedule errors are non-fatal — PopulateMatches warns and continues.
	if err := api.PopulateMatches(bg(), 1, "test_round", true); err != nil {
		t.Errorf("expected nil when schedule fetch fails (non-fatal), got: %v", err)
	}
}

func TestPopulateMatches_ResultsError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchAndSaveMatchResultsError = fmt.Errorf("results fetch failed")
	api := &App{Store: mockStore, rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10)}

	if err := api.PopulateMatches(bg(), 1, "test_round", false); err == nil {
		t.Error("expected error when results fetch fails, got nil")
	}
}

func TestPopulateMatches_RateLimiterNil(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := &App{Store: mockStore, rateLimiter: nil}

	err := api.PopulateMatches(bg(), 1, "test_round", true)
	if err == nil || !strings.Contains(err.Error(), "rate limiter") {
		t.Errorf("expected rate limiter error, got: %v", err)
	}
}

func TestPopulateMatches_RateLimitExceeded(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	limiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	limiter.Allow()
	api := &App{Store: mockStore, rateLimiter: limiter}

	err := api.PopulateMatches(bg(), 1, "test_round", true)
	if err == nil || !strings.Contains(err.Error(), "rate limiter limit reached") {
		t.Errorf("expected rate limiter error, got: %v", err)
	}
}

// endregion

// region UpdateMatchSchedule

func TestUpdateMatchSchedule_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := &App{Store: mockStore, rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10)}

	if err := api.UpdateMatchSchedule(bg(), 1); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestUpdateMatchSchedule_RateLimitExceeded(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	limiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	limiter.Allow()
	api := &App{Store: mockStore, rateLimiter: limiter}

	err := api.UpdateMatchSchedule(bg(), 1)
	if err == nil || !strings.Contains(err.Error(), "rate limiter exceeded") {
		t.Errorf("expected rate limiter error, got: %v", err)
	}
}

func TestUpdateMatchSchedule_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchAndSaveScheduleError = fmt.Errorf("schedule fetch failed")
	api := &App{Store: mockStore, rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10)}

	err := api.UpdateMatchSchedule(bg(), 1)
	if err == nil || !strings.Contains(err.Error(), "schedule fetch failed") {
		t.Errorf("expected store error, got: %v", err)
	}
}

// endregion

// region StoreSchedule

func TestStoreSchedule_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := NewTestApp(mockStore)

	matches := []sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B", EpochTime: 1000, BestOf: "3"}}
	if err := api.StoreSchedule(bg(), 1, matches); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestStoreSchedule_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.UpsertMatchScheduleError = fmt.Errorf("db write failed")
	api := NewTestApp(mockStore)

	err := api.StoreSchedule(bg(), 1, []sources.ScheduledMatch{{Team1: "A", Team2: "B", EpochTime: 1000}})
	if err == nil || !strings.Contains(err.Error(), "db write failed") {
		t.Errorf("expected store error, got: %v", err)
	}
}

// endregion

// region UpdateMatchResults

func TestUpdateMatchResults_Success(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := &App{Store: mockStore, rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10)}

	if err := api.UpdateMatchResults(bg(), 1, "test_round"); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestUpdateMatchResults_RateLimiterNil(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := &App{Store: mockStore, rateLimiter: nil}

	err := api.UpdateMatchResults(bg(), 1, "test_round")
	if err == nil || !strings.Contains(err.Error(), "rate limiter") {
		t.Errorf("expected rate limiter error, got: %v", err)
	}
}

func TestUpdateMatchResults_RateLimitExceeded(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	limiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	limiter.Allow()
	api := &App{Store: mockStore, rateLimiter: limiter}

	err := api.UpdateMatchResults(bg(), 1, "test_round")
	if err == nil || !strings.Contains(err.Error(), "rate limiter exceeded") {
		t.Errorf("expected rate limiter error, got: %v", err)
	}
}

func TestUpdateMatchResults_StoreError(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.FetchAndSaveMatchResultsError = fmt.Errorf("store error")
	api := &App{Store: mockStore, rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10)}

	err := api.UpdateMatchResults(bg(), 1, "test_round")
	if err == nil || !strings.Contains(err.Error(), "store error") {
		t.Errorf("expected store error, got: %v", err)
	}
}

// endregion

// region misc

func TestNewTestApp_HasRateLimiter(t *testing.T) {
	a := NewTestApp(NewMockStore("swiss", "test_round"))
	if !a.Allow() {
		t.Error("expected NewTestApp to return an App with a working rate limiter")
	}
}

func TestApp_Logger_NilLogger_ReturnsDefault(t *testing.T) {
	a := &App{}
	if a.logger() == nil {
		t.Error("expected non-nil logger when no logger injected")
	}
}

func TestMockStore_SetEliminationResults(t *testing.T) {
	mockStore := NewMockStore("single-elimination", "test_round")
	mockStore.SetEliminationResults(map[string]models.TeamProgress{"Team A": {Round: "final", Status: "advanced"}})
	if mockStore.MatchResults == nil {
		t.Error("expected MatchResults to be set")
	}
}

// endregion
