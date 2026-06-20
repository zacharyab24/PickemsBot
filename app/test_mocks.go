/* test_mocks.go
 * MockStore implementing store.Interface for unit tests.
 * Authors: Zachary Bower
 */

package app

import (
	"context"
	"fmt"
	"strings"

	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/store"
	"pickems-bot/tournament"

	"golang.org/x/time/rate"
)

// MockStore implements store.Interface for testing.
type MockStore struct {
	Predictions      map[string]models.Prediction
	MatchResults     tournament.MatchResult
	ScheduledMatches []sources.ScheduledMatch
	ValidTeams       []string
	Format           tournament.Kind
	VRSEntries       []store.VRSEntry
	Leaderboard      []store.LeaderboardEntry
	MatchNodes       []sources.MatchNode
	MatchKind        tournament.Kind
	GuildConfig      store.GuildConfig

	// Error injection
	EnsureTournamentError         error
	PingError                     error
	GetGuildConfigError           error
	UpsertGuildConfigError        error
	EnsureScheduledMatchesError   error
	ListValidTeamsError           error
	GetMatchResultsError          error
	UpsertMatchResultsError       error
	FetchAndSaveMatchResultsError error
	GetMatchNodesError            error
	GetMatchScheduleError         error
	UpsertMatchScheduleError      error
	FetchAndSaveScheduleError     error
	UpsertPredictionError         error
	GetPredictionError            error
	GetPredictionByUsernameError  error
	ListPredictionsError          error
	GetLeaderboardError           error
	ListVRSRankingsError          error

	StoreMatchScheduleCallCount int
}

// NewMockStore creates a MockStore pre-wired for the given format and round.
func NewMockStore(kind tournament.Kind, round string) *MockStore {
	tournamentID := 1
	cfg := store.GuildConfig{
		GuildID:      "test_guild",
		TournamentID: &tournamentID,
		Round:        &round,
	}
	return &MockStore{
		Predictions:      make(map[string]models.Prediction),
		ScheduledMatches: []sources.ScheduledMatch{},
		ValidTeams:       []string{"Team A", "Team B", "Team C", "Team D", "Team E", "Team F", "Team G", "Team H", "Team I", "Team J", "Team K", "Team L", "Team M", "Team N", "Team O", "Team P"},
		Format:           kind,
		GuildConfig:      cfg,
	}
}

// EnsureTournament implements store.Interface.
func (m *MockStore) EnsureTournament(_ context.Context, _, _, _ string, _ int) (int, error) {
	if m.EnsureTournamentError != nil {
		return 0, m.EnsureTournamentError
	}
	return 1, nil
}

// Ping implements store.Interface.
func (m *MockStore) Ping(ctx context.Context) error { return m.PingError }

// Close implements store.Interface.
func (m *MockStore) Close() {}

// GetGuildConfig implements store.Interface.
func (m *MockStore) GetGuildConfig(ctx context.Context, guildID, channelID string) (store.GuildConfig, error) {
	if m.GetGuildConfigError != nil {
		return store.GuildConfig{}, m.GetGuildConfigError
	}
	return m.GuildConfig, nil
}

// UpsertGuildConfig implements store.Interface.
func (m *MockStore) UpsertGuildConfig(ctx context.Context, cfg store.GuildConfig) error {
	return m.UpsertGuildConfigError
}

// EnsureScheduledMatches implements store.Interface.
func (m *MockStore) EnsureScheduledMatches(ctx context.Context, tournamentID int) error {
	if m.EnsureScheduledMatchesError != nil {
		return m.EnsureScheduledMatchesError
	}
	if len(m.ScheduledMatches) == 0 {
		return fmt.Errorf("no scheduled matches found")
	}
	return nil
}

// ListValidTeams implements store.Interface.
func (m *MockStore) ListValidTeams(ctx context.Context, tournamentID int, round string) ([]string, tournament.Kind, error) {
	if m.ListValidTeamsError != nil {
		return nil, "", m.ListValidTeamsError
	}
	return m.ValidTeams, m.Format, nil
}

// GetMatchResults implements store.Interface.
func (m *MockStore) GetMatchResults(ctx context.Context, tournamentID int, round string) (tournament.MatchResult, error) {
	if m.GetMatchResultsError != nil {
		return nil, m.GetMatchResultsError
	}
	if m.MatchResults == nil {
		return tournament.SwissResult{Teams: make(map[string]string)}, nil
	}
	return m.MatchResults, nil
}

// UpsertMatchResults implements store.Interface.
func (m *MockStore) UpsertMatchResults(ctx context.Context, tournamentID int, result tournament.MatchResult) error {
	return m.UpsertMatchResultsError
}

// FetchAndSaveMatchResults implements store.Interface.
func (m *MockStore) FetchAndSaveMatchResults(ctx context.Context, tournamentID int, round string) error {
	return m.FetchAndSaveMatchResultsError
}

// GetMatchNodes implements store.Interface.
func (m *MockStore) GetMatchNodes(ctx context.Context, tournamentID int, round string) ([]sources.MatchNode, tournament.Kind, error) {
	if m.GetMatchNodesError != nil {
		return nil, "", m.GetMatchNodesError
	}
	return m.MatchNodes, m.MatchKind, nil
}

// GetMatchSchedule implements store.Interface.
func (m *MockStore) GetMatchSchedule(ctx context.Context, tournamentID int) ([]sources.ScheduledMatch, error) {
	if m.GetMatchScheduleError != nil {
		return nil, m.GetMatchScheduleError
	}
	return m.ScheduledMatches, nil
}

// UpsertMatchSchedule implements store.Interface.
func (m *MockStore) UpsertMatchSchedule(ctx context.Context, tournamentID int, matches []sources.ScheduledMatch) error {
	m.StoreMatchScheduleCallCount++
	if m.UpsertMatchScheduleError != nil {
		return m.UpsertMatchScheduleError
	}
	m.ScheduledMatches = matches
	return nil
}

// FetchAndSaveSchedule implements store.Interface.
func (m *MockStore) FetchAndSaveSchedule(ctx context.Context, tournamentID int) error {
	return m.FetchAndSaveScheduleError
}

// UpsertPrediction implements store.Interface.
func (m *MockStore) UpsertPrediction(ctx context.Context, guildID string, tournamentID int, prediction models.Prediction) error {
	if m.UpsertPredictionError != nil {
		return m.UpsertPredictionError
	}
	m.Predictions[prediction.UserID] = prediction
	return nil
}

// GetPrediction implements store.Interface.
func (m *MockStore) GetPrediction(ctx context.Context, userID, guildID string, tournamentID int, round string) (models.Prediction, error) {
	if m.GetPredictionError != nil {
		return models.Prediction{}, m.GetPredictionError
	}
	pred, ok := m.Predictions[userID]
	if !ok {
		return models.Prediction{}, fmt.Errorf("prediction not found for user %s", userID)
	}
	return pred, nil
}

// GetPredictionByUsername implements store.Interface.
func (m *MockStore) GetPredictionByUsername(ctx context.Context, username, guildID string, tournamentID int, round string) (models.Prediction, error) {
	if m.GetPredictionByUsernameError != nil {
		return models.Prediction{}, m.GetPredictionByUsernameError
	}
	lower := strings.ToLower(username)
	for _, pred := range m.Predictions {
		if strings.ToLower(pred.Username) == lower {
			return pred, nil
		}
	}
	return models.Prediction{}, fmt.Errorf("prediction not found for username %s", username)
}

// ListPredictions implements store.Interface.
func (m *MockStore) ListPredictions(ctx context.Context, guildID string, tournamentID int, round string) ([]models.Prediction, error) {
	if m.ListPredictionsError != nil {
		return nil, m.ListPredictionsError
	}
	var out []models.Prediction
	for _, p := range m.Predictions {
		out = append(out, p)
	}
	return out, nil
}

// GetLeaderboard implements store.Interface.
func (m *MockStore) GetLeaderboard(ctx context.Context, guildID string, tournamentID int) ([]store.LeaderboardEntry, error) {
	if m.GetLeaderboardError != nil {
		return nil, m.GetLeaderboardError
	}
	return m.Leaderboard, nil
}

// ListVRSRankings implements store.Interface.
func (m *MockStore) ListVRSRankings(ctx context.Context) ([]store.VRSEntry, error) {
	if m.ListVRSRankingsError != nil {
		return nil, m.ListVRSRankingsError
	}
	return m.VRSEntries, nil
}

// --- Test setup helpers ---

// SetSwissResults configures the mock to return a SwissResult for GetMatchResults calls.
func (m *MockStore) SetSwissResults(scores map[string]string) {
	round := ""
	if m.GuildConfig.Round != nil {
		round = *m.GuildConfig.Round
	}
	m.MatchResults = tournament.SwissResult{Round: round, Teams: scores}
}

// SetEliminationResults configures the mock to return an EliminationResult for GetMatchResults calls.
func (m *MockStore) SetEliminationResults(progression map[string]models.TeamProgress) {
	round := ""
	if m.GuildConfig.Round != nil {
		round = *m.GuildConfig.Round
	}
	m.MatchResults = tournament.EliminationResult{Round: round, Teams: progression}
	m.Format = tournament.SingleElim
	m.ValidTeams = make([]string, 0, len(progression))
	for team := range progression {
		m.ValidTeams = append(m.ValidTeams, team)
	}
}

// SetScheduledMatches sets the scheduled matches returned by GetMatchSchedule and EnsureScheduledMatches.
func (m *MockStore) SetScheduledMatches(matches []sources.ScheduledMatch) {
	m.ScheduledMatches = matches
}

// SetVRSEntries sets the VRS entries returned by ListVRSRankings.
func (m *MockStore) SetVRSEntries(entries []store.VRSEntry) {
	m.VRSEntries = entries
}

// NewTestApp creates a minimal App for unit tests with an unlimited rate limiter.
func NewTestApp(s store.Interface) *App {
	return &App{
		Store:       s,
		rateLimiter: rate.NewLimiter(rate.Inf, 1),
	}
}
