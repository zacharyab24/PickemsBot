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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

// Data source identifiers, as stored in tournaments.source.
const (
	sourcePandaScore = "pandascore"
	sourceLiquipedia = "liquipedia"
)

// App provides methods for interacting with the pickems bot data layer.
type App struct {
	Store store.Interface
	// Each data source gets its own limiter - guild_config is source-agnostic
	// (see resolveFetcher below), so a single deployment can end up fetching
	// tournaments of either source, each with its own rate limits to respect.
	pandaScoreLimiter *rate.Limiter
	liquipediaLimiter *rate.Limiter
	log               *slog.Logger
	MonitoringPool    *MonitoringPool

	// configLocks serialises the read-modify-write sequence in
	// SetConfigTournament/SetConfigRound per (guildID, channelID), so two
	// near-simultaneous /config changes for the same channel can't interleave
	// their guild_config read with each other's write.
	configLocks sync.Map // map[string]*sync.Mutex
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
	switch cfg.DataSource {
	case sourceLiquipedia, sourcePandaScore:
	default:
		return nil, fmt.Errorf("unsupported data source: %s", cfg.DataSource)
	}

	// Both limiters are always built, regardless of cfg.DataSource -
	// resolveFetcher below is source-agnostic, so a deployment configured for
	// one source can still end up fetching tournaments of the other.
	pandaScoreLimiter := rate.NewLimiter(rate.Every(4*time.Second), 5)
	liquipediaLimiter := rate.NewLimiter(rate.Every(time.Minute), 10)

	// resolveFetcher builds a fetcher from the tournament actually being
	// fetched, not from cfg.DataSource - guild_config is source-agnostic, so a
	// store can end up fetching for tournaments of either source.
	pandaScoreAPIURL, pandaScoreAPIKey := cfg.PandaScore.APIURL, os.Getenv("PANDASCORE_API_KEY")
	liquipediaAPIURL, liquipediaAPIKey := cfg.Liquipedia.APIURL, os.Getenv("LIQUIDPEDIADB_API_KEY")
	resolveFetcher := func(t store.Tournament) (store.DataSourceFetcher, error) {
		switch t.Source {
		case sourcePandaScore:
			extID, seriesID, err := parsePandaScoreIDs(t)
			if err != nil {
				return nil, err
			}
			return store.NewPandaScoreFetcher(pandaScoreAPIURL, pandaScoreAPIKey, seriesID, extID), nil
		case sourceLiquipedia:
			return store.NewLiquipediaFetcher(liquipediaAPIURL, liquipediaAPIKey, t.ExternalID), nil
		default:
			return nil, fmt.Errorf("unsupported tournament source: %s", t.Source)
		}
	}

	var appLog, storeLog *slog.Logger
	if log != nil {
		appLog = log.With("component", "app")
		storeLog = log.With("component", "store")
	}

	s, err := store.NewStore(postgresURI, resolveFetcher, storeLog)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	return &App{
		Store:             s,
		pandaScoreLimiter: pandaScoreLimiter,
		liquipediaLimiter: liquipediaLimiter,
		log:               appLog,
		MonitoringPool:    newMonitoringPool(),
	}, nil
}

// parsePandaScoreIDs parses a tournament's PandaScore external and series ids
// from their stored string form. extID is mandatory - a PandaScore tournament
// always has one. seriesID is optional: empty, or unparseable, resolves to 0
// (unscoped) rather than erroring, matching PandaScore's own optional series
// filter - a tournament shouldn't silently drop out of monitoring just because
// its series id is unset.
func parsePandaScoreIDs(t store.Tournament) (extID, seriesID int, err error) {
	extID, err = strconv.Atoi(t.ExternalID)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid pandascore external id %q: %w", t.ExternalID, err)
	}
	seriesID, _ = strconv.Atoi(t.SeriesID) // optional - 0 if unset or invalid
	return extID, seriesID, nil
}

// limiterFor returns the rate limiter paced for source ("pandascore" or
// "liquipedia").
func (a *App) limiterFor(source string) *rate.Limiter {
	if source == sourceLiquipedia {
		return a.liquipediaLimiter
	}
	return a.pandaScoreLimiter
}

// Allow calls source's configured rate limiter's Allow() function.
// Non-blocking: returns false immediately when no token is available. The poller
// uses this to skip a tick rather than wait.
func (a *App) Allow(source string) bool {
	l := a.limiterFor(source)
	if l == nil {
		return false
	}
	return l.Allow()
}

// Wait blocks until source's rate limiter permits another call, or ctx is
// cancelled. Backed by the same limiter as Allow(source); batch jobs use this
// to pace instead of drop.
func (a *App) Wait(ctx context.Context, source string) error {
	l := a.limiterFor(source)
	if l == nil {
		return nil
	}
	return l.Wait(ctx)
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

	// Unlike SetUserPrediction/CheckPrediction, no EnsureScheduledMatches guard
	// here - "nothing scheduled yet" is a legitimate, answerable state (an
	// empty list), not a failure.
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
// source picks which rate limiter paces the fetch - see App.Allow.
func (a *App) PopulateMatches(ctx context.Context, tournamentID int, round, source string, scheduleOnly bool) error {
	if !a.Allow(source) {
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
// source picks which rate limiter paces the fetch - see App.Allow.
func (a *App) UpdateMatchSchedule(ctx context.Context, tournamentID int, source string) error {
	if !a.Allow(source) {
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
// source picks which rate limiter paces the fetch - see App.Allow.
func (a *App) UpdateMatchResults(ctx context.Context, tournamentID int, round, source string) error {
	if !a.Allow(source) {
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

// GetConfig returns the raw guild config for a channel for `/config view`.
// Unlike resolveConfig, this does not require a tournament or round to be set.
func (a *App) GetConfig(ctx context.Context, guildID, channelID string) (store.GuildConfig, error) {
	return a.Store.GetGuildConfig(ctx, guildID, channelID)
}

// SetConfigTournament points a guild/channel at a specific tournament stage,
// identified by the (name, round) pair the admin picked in /config set-tournament.
// Returns the resolved tournament for the confirmation message. An unrecognised
// pair (e.g. free-typed text that matched no row) surfaces as an error.
func (a *App) SetConfigTournament(ctx context.Context, guildID, channelID, name, round string) (store.Tournament, error) {
	lock := a.configLock(guildID, channelID)
	lock.Lock()
	defer lock.Unlock()

	// Read the pre-change config so updatePool can unsubscribe the old
	// tournament, if any, once the switch is made.
	prev, err := a.Store.GetGuildConfig(ctx, guildID, channelID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return store.Tournament{}, fmt.Errorf("SetConfigTournament: %w", err)
	}

	t, err := a.Store.GetTournamentByNameAndRound(ctx, name, round)
	if err != nil {
		return store.Tournament{}, fmt.Errorf("SetConfigTournament: %w", err)
	}

	if err := a.upsertConfigField(ctx, guildID, channelID, prev, func(c *store.GuildConfig) {
		c.TournamentID = &t.ID
		c.Round = &t.Round
	}); err != nil {
		return store.Tournament{}, err
	}

	// Synchronous, still under the lock: cheap (one query plus in-memory map
	// ops), and keeps the whole read-upsert-updatePool sequence atomic with
	// respect to another /config change on this same guild/channel.
	a.updatePool(ctx, prev, t.ID)

	go a.finishConfigChange(t)
	return t, nil
}

// ListTournamentNames returns the distinct tournament names for the /config
// set-tournament picklist (one entry per tournament, not per round).
func (a *App) ListTournamentNames(ctx context.Context) ([]string, error) {
	return a.Store.ListTournamentNames(ctx)
}

// ListRoundsForTournament returns the rounds available for a tournament name,
// scoping the round autocomplete to what the chosen tournament actually offers.
func (a *App) ListRoundsForTournament(ctx context.Context, name string) ([]string, error) {
	return a.Store.ListRoundsForTournament(ctx, name)
}

// RoundsForConfiguredTournament returns the rounds available for the tournament
// currently configured on this guild/channel. Errors if no tournament is
// configured yet.
func (a *App) RoundsForConfiguredTournament(ctx context.Context, guildID, channelID string) ([]string, error) {
	cfg, err := a.Store.GetGuildConfig(ctx, guildID, channelID)
	if err != nil {
		return nil, fmt.Errorf("RoundsForConfiguredTournament: %w", err)
	}
	if cfg.TournamentID == nil {
		return nil, fmt.Errorf("RoundsForConfiguredTournament: no tournament configured")
	}
	t, err := a.Store.GetTournament(ctx, *cfg.TournamentID)
	if err != nil {
		return nil, fmt.Errorf("RoundsForConfiguredTournament: %w", err)
	}
	return a.Store.ListRoundsForTournament(ctx, t.Name)
}

// SetConfigRound changes only the round for this guild/channel, keeping the
// same tournament. Each (name, round) pair is a distinct row, so this resolves
// the current tournament's name plus the new round to that row and stores both.
// Errors if no tournament is configured, or the round isn't valid for it.
func (a *App) SetConfigRound(ctx context.Context, guildID, channelID, round string) (store.Tournament, error) {
	lock := a.configLock(guildID, channelID)
	lock.Lock()
	defer lock.Unlock()

	cfg, err := a.Store.GetGuildConfig(ctx, guildID, channelID)
	if err != nil {
		return store.Tournament{}, fmt.Errorf("SetConfigRound: %w", err)
	}
	if cfg.TournamentID == nil {
		return store.Tournament{}, fmt.Errorf("SetConfigRound: no tournament configured")
	}
	current, err := a.Store.GetTournament(ctx, *cfg.TournamentID)
	if err != nil {
		return store.Tournament{}, fmt.Errorf("SetConfigRound: %w", err)
	}
	t, err := a.Store.GetTournamentByNameAndRound(ctx, current.Name, round)
	if err != nil {
		return store.Tournament{}, fmt.Errorf("SetConfigRound: %w", err)
	}

	if err := a.upsertConfigField(ctx, guildID, channelID, cfg, func(c *store.GuildConfig) {
		c.TournamentID = &t.ID
		c.Round = &t.Round
	}); err != nil {
		return store.Tournament{}, err
	}

	a.updatePool(ctx, cfg, t.ID) // see SetConfigTournament - same locking/ordering reasoning
	go a.finishConfigChange(t)

	return t, nil
}

// upsertConfigField applies mutate to base - the caller's own already-fetched
// guild config, or a zero-value store.GuildConfig{} for first-time setup -
// and upserts the result. Callers already need that row themselves (e.g. to
// diff against for updatePool), so this takes it instead of fetching its own
// copy of the same row a moment later.
func (a *App) upsertConfigField(ctx context.Context, guildID, channelID string, base store.GuildConfig, mutate func(*store.GuildConfig)) error {
	cfg := base

	// results_channel_id is the ON CONFLICT key - must be set for the upsert to match.
	cfg.GuildID = guildID
	cfg.ResultsChannelID = &channelID

	mutate(&cfg) // caller's one-field change

	// guild_config.guild_id has an FK to guilds - ensure the parent row first.
	if err := a.Store.EnsureGuild(ctx, guildID); err != nil {
		return fmt.Errorf("upsertConfigField: %w", err)
	}
	if err := a.Store.UpsertGuildConfig(ctx, cfg); err != nil {
		return fmt.Errorf("upsertConfigField: %w", err)
	}
	return nil
}

// configLock returns the mutex serialising config changes for one
// (guildID, channelID) pair, creating it on first use.
func (a *App) configLock(guildID, channelID string) *sync.Mutex {
	v, _ := a.configLocks.LoadOrStore(guildID+":"+channelID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// finishConfigChange runs the slow, best-effort follow-up work after a config
// change: format detection, then an initial populate. Uses a detached context
// since the caller's request context may be cancelled once it responds.
//
// The two run sequentially, not as separate goroutines - populate reads
// t.Format fresh from the DB, so if it started concurrently with detection it
// could see a still-NULL format and fail to detect a kind that detection was
// about to persist a moment later. Running them one after another also means
// they no longer race each other for the same rate limiter's tokens.
func (a *App) finishConfigChange(t store.Tournament) {
	if err := a.checkAndStoreFormat(context.Background(), t); err != nil {
		a.logger().Warn("format detection failed",
			"tournament_id", t.ID, "external_id", t.ExternalID, "error", err)
	}
	if err := a.PopulateMatches(context.Background(), t.ID, t.Round, t.Source, false); err != nil {
		a.logger().Warn("initial populate failed - results for matches already finished before tracking began may be missed until manually retried",
			"tournament_id", t.ID, "round", t.Round, "error", err)
	}
}

// updatePool reconciles the monitoring pool with a guild's config change.
// If prev pointed at a different tournament, that tournament is unsubscribed
// only once TournamentStillReferenced confirms no other guild_config row
// still points at it; on error it's left subscribed rather than risking an
// early drop. newTournamentID is always subscribed.
func (a *App) updatePool(ctx context.Context, prev store.GuildConfig, newTournamentID int) {
	if prev.TournamentID != nil && *prev.TournamentID != newTournamentID {
		stillReferenced, err := a.Store.TournamentStillReferenced(ctx, *prev.TournamentID)
		if err != nil {
			a.logger().Warn("failed to check if tournament is still referenced, leaving it subscribed",
				"tournament_id", *prev.TournamentID, "error", err)
		} else if !stillReferenced {
			a.Unsubscribe(ctx, *prev.TournamentID)
		}
	}
	a.Subscribe(ctx, newTournamentID)
}

// checkAndStoreFormat fetches the PandaScore bracket for a tournament, infers its format kind, and persists that value in the db.
func (a *App) checkAndStoreFormat(ctx context.Context, t store.Tournament) error {
	if t.Source != sourcePandaScore || t.Format != nil {
		return nil // only PandaScore has a format field, and it's already set
	}

	// external_id is stored as text but the bracket endpoint keys on the numeric
	// PandaScore id; parse before spending a rate-limiter token.
	extID, _, err := parsePandaScoreIDs(t)
	if err != nil {
		return fmt.Errorf("checkAndStoreFormat: %w", err)
	}

	if err := a.Wait(ctx, t.Source); err != nil {
		return err
	}

	brackets, err := sources.GetPandaScoreBracket(os.Getenv("PANDASCORE_API_KEY"), extID)
	if err != nil {
		return fmt.Errorf("checkAndStoreFormat: %w", err)
	}
	kind := tournament.DetectKindFromBracket(brackets, sources.CountTeams(brackets))
	return a.Store.SetTournamentFormat(ctx, t.ID, string(kind))
}
