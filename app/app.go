/* app.go
 * Public methods for interacting with the pickems bot data layer.
 * User-facing commands take (ctx, guildID, channelID) and resolve tournament context from guild_config.
 * Background/poller operations take explicit (tournamentID, round) parameters.
 * Authors: Zachary Bower
 */

package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"pickems-bot/config"
	"pickems-bot/metrics"
	"pickems-bot/models"
	"pickems-bot/scoring"
	"pickems-bot/sources"
	"pickems-bot/store"
	"pickems-bot/tournament"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

// App provides methods for interacting with the pickems bot data layer.
type App struct {
	Store       store.Interface
	rateLimiter *rate.Limiter
	log         *slog.Logger
}

func (a *App) logger() *slog.Logger {
	if a.log == nil {
		return slog.Default()
	}
	return a.log
}

// NewApp creates a new App instance.
// postgresURI is the connection string for the PostgreSQL database.
// log may be nil; if so the global slog default is used.
func NewApp(cfg config.Config, postgresURI string, log *slog.Logger) (*App, error) {
	var fetcher store.DataSourceFetcher
	var limiter *rate.Limiter
	switch cfg.DataSource {
	case "liquipedia":
		fetcher = store.NewLiquipediaFetcher(cfg.Liquipedia.APIURL, os.Getenv("LIQUIDPEDIADB_API_KEY"), cfg.Liquipedia.Page)
		limiter = rate.NewLimiter(rate.Every(time.Minute), 10)

	case "pandascore":
		fetcher = store.NewPandaScoreFetcher(cfg.PandaScore.APIURL, os.Getenv("PANDASCORE_API_KEY"), cfg.PandaScore.SeriesID, cfg.PandaScore.TournamentID)
		limiter = rate.NewLimiter(rate.Every(4*time.Second), 5)

	default:
		return nil, fmt.Errorf("unsupported data source: %s", cfg.DataSource)
	}

	var appLog, storeLog *slog.Logger
	if log != nil {
		appLog = log.With("component", "app")
		storeLog = log.With("component", "store")
	}

	s, err := store.NewStore(postgresURI, fetcher, storeLog)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	return &App{
		Store:       s,
		rateLimiter: limiter,
		log:         appLog,
	}, nil
}

// Allow calls the app's configured rate limiter's Allow() function.
func (a *App) Allow() bool {
	if a.rateLimiter == nil {
		return false
	}
	return a.rateLimiter.Allow()
}

// resolveConfig looks up the guild config and validates that a tournament and round are set.
func (a *App) resolveConfig(ctx context.Context, guildID, channelID string) (store.GuildConfig, error) {
	cfg, err := a.Store.GetGuildConfig(ctx, guildID, channelID)
	if err != nil {
		return store.GuildConfig{}, fmt.Errorf("no configuration found for this server/channel: %w", err)
	}
	if cfg.TournamentID == nil || cfg.Round == nil {
		return store.GuildConfig{}, errors.New("tournament not configured for this server — use /config to set up")
	}
	return cfg, nil
}

// SetUserPrediction validates and stores a user's prediction for the configured tournament round.
func (a *App) SetUserPrediction(ctx context.Context, guildID, channelID string, user models.User, inputTeams []string) (models.Prediction, error) {
	cfg, err := a.resolveConfig(ctx, guildID, channelID)
	if err != nil {
		return models.Prediction{}, err
	}

	if err := a.Store.EnsureScheduledMatches(ctx, *cfg.TournamentID); err != nil {
		return models.Prediction{}, err
	}

	validTeams, formatName, err := a.Store.ListValidTeams(ctx, *cfg.TournamentID, *cfg.Round)
	if err != nil {
		return models.Prediction{}, err
	}

	f, err := tournament.Get(formatName)
	if err != nil {
		return models.Prediction{}, fmt.Errorf("unknown tournament format: %s", formatName)
	}

	requiredPredictions := f.RequiredPredictions(len(validTeams))
	if len(inputTeams) != requiredPredictions {
		return models.Prediction{}, fmt.Errorf("incorrect number of teams arguments, expected %d but got %d", requiredPredictions, len(inputTeams))
	}

	for i := range inputTeams {
		inputTeams[i] = strings.ReplaceAll(inputTeams[i], "\"", "")
		inputTeams[i] = strings.ReplaceAll(inputTeams[i], "“", "")
		inputTeams[i] = strings.ReplaceAll(inputTeams[i], "”", "")
	}

	teams, invalidTeams := scoring.CheckTeamNames(inputTeams, validTeams)
	if len(invalidTeams) > 0 {
		var str strings.Builder
		str.WriteString("the following team names are invalid:")
		for i := range invalidTeams {
			str.WriteString(fmt.Sprintf(" '%s'", invalidTeams[i]))
		}
		return models.Prediction{}, errors.New(str.String())
	}

	seen := make(map[string]string)
	for i, team := range teams {
		if original, exists := seen[team]; exists {
			if original == inputTeams[i] {
				return models.Prediction{}, fmt.Errorf("'%s' entered multiple times, stored prediction was not updated", team)
			}
			return models.Prediction{}, fmt.Errorf("'%s' and '%s' both resolved to '%s'. Please enter a more specific name for one of them", original, inputTeams[i], team)
		}
		seen[team] = inputTeams[i]
	}

	prediction, err := f.GeneratePrediction(user, *cfg.Round, teams)
	if err != nil {
		return models.Prediction{}, err
	}

	if err := a.Store.UpsertPrediction(ctx, guildID, *cfg.TournamentID, prediction); err != nil {
		return models.Prediction{}, err
	}

	return prediction, nil
}

// CheckPrediction fetches and scores a user's stored prediction.
func (a *App) CheckPrediction(ctx context.Context, guildID, channelID string, user models.User) (tournament.ScoreReport, error) {
	cfg, err := a.resolveConfig(ctx, guildID, channelID)
	if err != nil {
		return nil, err
	}

	if err := a.Store.EnsureScheduledMatches(ctx, *cfg.TournamentID); err != nil {
		return nil, err
	}

	doc, err := a.Store.GetPrediction(ctx, user.UserID, guildID, *cfg.TournamentID, *cfg.Round)
	if err != nil {
		return nil, err
	}

	results, err := a.Store.GetMatchResults(ctx, *cfg.TournamentID, *cfg.Round)
	if err != nil {
		return nil, err
	}

	return scoring.CalculateUserScore(doc, results)
}

// CheckPredictionByUsername looks up picks by username (case-insensitive) and scores them.
func (a *App) CheckPredictionByUsername(ctx context.Context, guildID, channelID, username string) (models.User, tournament.ScoreReport, error) {
	cfg, err := a.resolveConfig(ctx, guildID, channelID)
	if err != nil {
		return models.User{}, nil, err
	}

	if err := a.Store.EnsureScheduledMatches(ctx, *cfg.TournamentID); err != nil {
		return models.User{}, nil, err
	}

	doc, err := a.Store.GetPredictionByUsername(ctx, username, guildID, *cfg.TournamentID, *cfg.Round)
	if err != nil {
		return models.User{}, nil, err
	}

	results, err := a.Store.GetMatchResults(ctx, *cfg.TournamentID, *cfg.Round)
	if err != nil {
		return models.User{}, nil, err
	}

	report, err := scoring.CalculateUserScore(doc, results)
	if err != nil {
		return models.User{}, nil, err
	}

	return models.User{UserID: doc.UserID, Username: doc.Username}, report, nil
}

// GetLeaderboard returns the ranked leaderboard for the guild's configured tournament.
// Scores are materialised on match result insert, so this is a simple read.
func (a *App) GetLeaderboard(ctx context.Context, guildID, channelID string) ([]LeaderboardUser, error) {
	cfg, err := a.resolveConfig(ctx, guildID, channelID)
	if err != nil {
		return nil, err
	}

	entries, err := a.Store.GetLeaderboard(ctx, guildID, *cfg.TournamentID)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Successes > entries[j].Successes
	})

	response := make([]LeaderboardUser, 0, len(entries))
	for i, e := range entries {
		response = append(response, LeaderboardUser{
			Username:  e.Username,
			Rank:      i + 1,
			Successes: e.Successes,
			Failures:  e.Failed,
		})
	}
	return response, nil
}

// GetTeams returns all valid teams for the round with their VRS world rankings.
func (a *App) GetTeams(ctx context.Context, guildID, channelID string) ([]Team, error) {
	cfg, err := a.resolveConfig(ctx, guildID, channelID)
	if err != nil {
		return nil, err
	}

	validTeams, _, err := a.Store.ListValidTeams(ctx, *cfg.TournamentID, *cfg.Round)
	if err != nil {
		return nil, err
	}

	vrsEntries, err := a.Store.ListVRSRankings(ctx)
	if err != nil {
		return nil, err
	}

	vrsNorm := make(map[string]int, len(vrsEntries))
	vrsNormKeys := make([]string, 0, len(vrsEntries))
	for _, entry := range vrsEntries {
		key := sources.NormalizeTeamName(entry.TeamName)
		vrsNorm[key] = entry.Standing
		vrsNormKeys = append(vrsNormKeys, key)
	}

	var result []Team
	for _, teamName := range validTeams {
		norm := sources.NormalizeTeamName(teamName)
		ranking, ok := vrsNorm[norm]
		if !ok {
			if matches := fuzzy.RankFind(norm, vrsNormKeys); len(matches) > 0 {
				ranking = vrsNorm[matches[0].Target]
			}
		}
		result = append(result, Team{Name: teamName, VRSRanking: ranking})
	}
	return result, nil
}

// GetTeam returns VRS data for a single team by name.
func (a *App) GetTeam(ctx context.Context, teamName string) (store.VRSEntry, error) {
	if teamName == "" {
		return store.VRSEntry{}, fmt.Errorf("cannot lookup empty team name")
	}

	vrsEntries, err := a.Store.ListVRSRankings(ctx)
	if err != nil {
		return store.VRSEntry{}, err
	}

	norm := sources.NormalizeTeamName(teamName)
	for _, entry := range vrsEntries {
		if sources.NormalizeTeamName(entry.TeamName) == norm {
			return entry, nil
		}
	}

	keys := make([]string, len(vrsEntries))
	for i, e := range vrsEntries {
		keys[i] = sources.NormalizeTeamName(e.TeamName)
	}
	if matches := fuzzy.RankFind(norm, keys); len(matches) > 0 {
		for _, entry := range vrsEntries {
			if sources.NormalizeTeamName(entry.TeamName) == matches[0].Target {
				return entry, nil
			}
		}
	}

	return store.VRSEntry{}, fmt.Errorf("no VRS data found for %q", teamName)
}

// GetUpcomingMatches returns non-finished scheduled matches for the guild's configured tournament.
func (a *App) GetUpcomingMatches(ctx context.Context, guildID, channelID string) ([]sources.ScheduledMatch, error) {
	cfg, err := a.resolveConfig(ctx, guildID, channelID)
	if err != nil {
		return nil, err
	}

	if err := a.Store.EnsureScheduledMatches(ctx, *cfg.TournamentID); err != nil {
		return nil, err
	}

	scheduledMatches, err := a.Store.GetMatchSchedule(ctx, *cfg.TournamentID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	var matches []sources.ScheduledMatch
	for _, match := range scheduledMatches {
		if match.Finished {
			continue
		}
		if match.EpochTime < now {
			match.Live = true
		}
		matches = append(matches, match)
	}

	slices.SortFunc(matches, func(a, b sources.ScheduledMatch) int {
		return cmp.Compare(a.EpochTime, b.EpochTime)
	})
	return matches, nil
}

// GetResults returns raw match nodes and format kind for the guild's active tournament round.
// Section labels are normalised: existing rows without a stored section fall back to positional
// derivation (single-elim) or Swiss record normalisation.
func (a *App) GetResults(ctx context.Context, guildID, channelID string) ([]sources.MatchNode, tournament.Kind, error) {
	cfg, err := a.resolveConfig(ctx, guildID, channelID)
	if err != nil {
		return nil, "", err
	}
	nodes, kind, err := a.Store.GetMatchNodes(ctx, *cfg.TournamentID, *cfg.Round)
	if err != nil {
		return nil, "", err
	}
	switch kind {
	case tournament.Swiss:
		nodes = tournament.NormalizeSwissSections(nodes)
	case tournament.SingleElim:
		nodes = tournament.NormalizeSingleElimSections(nodes)
	}
	return nodes, kind, nil
}

// GetTournamentInfo returns metadata about the guild's configured tournament.
func (a *App) GetTournamentInfo(ctx context.Context, guildID, channelID string) (TournamentInfo, error) {
	cfg, err := a.resolveConfig(ctx, guildID, channelID)
	if err != nil {
		return TournamentInfo{}, err
	}

	if err := a.Store.EnsureScheduledMatches(ctx, *cfg.TournamentID); err != nil {
		return TournamentInfo{}, err
	}

	validTeams, formatName, err := a.Store.ListValidTeams(ctx, *cfg.TournamentID, *cfg.Round)
	if err != nil {
		return TournamentInfo{}, err
	}

	f, err := tournament.Get(tournament.Kind(formatName))
	if err != nil {
		return TournamentInfo{}, err
	}

	name := ""
	if cfg.TournamentName != nil {
		name = *cfg.TournamentName
	}

	return TournamentInfo{
		TournamentName: name,
		Round:          *cfg.Round,
		Format:         string(formatName),
		NumTeams:       f.RequiredPredictions(len(validTeams)),
	}, nil
}

// PopulateMatches fetches and stores match schedule and optionally results for a specific tournament.
func (a *App) PopulateMatches(ctx context.Context, tournamentID int, round string, scheduleOnly bool) error {
	if !a.Allow() {
		return fmt.Errorf("rate limiter limit reached")
	}

	if err := a.Store.FetchAndSaveSchedule(ctx, tournamentID); err != nil {
		a.logger().Warn("PopulateMatches: schedule fetch skipped", "tournament_id", tournamentID, "error", err)
	}

	if !scheduleOnly {
		a.logger().Info("PopulateMatches: fetching results", "tournament_id", tournamentID, "round", round)
		if err := a.Store.FetchAndSaveMatchResults(ctx, tournamentID, round); err != nil {
			a.logger().Error("PopulateMatches: results fetch failed", "tournament_id", tournamentID, "round", round, "error", err)
			return err
		}
		a.logger().Info("PopulateMatches: results fetch complete", "tournament_id", tournamentID, "round", round)
	}
	return nil
}

// UpdateMatchSchedule is a rate-limited wrapper around Store.FetchAndSaveSchedule.
func (a *App) UpdateMatchSchedule(ctx context.Context, tournamentID int) error {
	if !a.Allow() {
		return fmt.Errorf("rate limiter exceeded, skipping match schedule update")
	}
	return a.Store.FetchAndSaveSchedule(ctx, tournamentID)
}

// StoreSchedule persists a pre-fetched schedule slice.
// Used by the PandaScore poller to reuse already-fetched data.
func (a *App) StoreSchedule(ctx context.Context, tournamentID int, matches []sources.ScheduledMatch) error {
	return a.Store.UpsertMatchSchedule(ctx, tournamentID, matches)
}

// UpdateMatchResults is a rate-limited wrapper around Store.FetchAndSaveMatchResults.
func (a *App) UpdateMatchResults(ctx context.Context, tournamentID int, round string) error {
	if !a.Allow() {
		return fmt.Errorf("rate limiter exceeded, skipping match result update")
	}
	timer := prometheus.NewTimer(metrics.LeaderboardDuration)
	defer timer.ObserveDuration()

	if err := a.Store.FetchAndSaveMatchResults(ctx, tournamentID, round); err != nil {
		return err
	}
	metrics.MatchUpdatesTotal.Inc()
	return nil
}
