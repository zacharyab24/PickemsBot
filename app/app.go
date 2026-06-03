/* app.go
 * This file contains the public methods for interacting with this package. For consistent results, fuctions should
 * only be called from this file, not the sub packages for match and processing. For details about functionality see `api.md`
 * Authors: Zachary Bower
 */

package app

import (
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
	"sort"
	"strings"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

// App provides methods for interacting with the pickems bot data layer
type App struct {
	Store       store.Interface
	rateLimiter *rate.Limiter
	log         *slog.Logger
}

// logger returns the app's logger, falling back to the global default when none was injected.
func (a *App) logger() *slog.Logger {
	if a.log == nil {
		return slog.Default()
	}
	return a.log
}

// NewApp creates a new App instance with the provided configuration.
// log may be nil; if so the global slog default is used.
func NewApp(cfg config.Config, mongoURI string, log *slog.Logger) (*App, error) {
	var fetcher store.DataSourceFetcher
	var limiter *rate.Limiter
	switch cfg.DataSource {
	case "liquipedia":
		fetcher = store.NewLiquipediaFetcher(cfg.Liquipedia.APIURL, os.Getenv("LIQUIDPEDIADB_API_KEY"), cfg.Liquipedia.Page)
		limiter = rate.NewLimiter(rate.Every(time.Minute), 10) // 60/hr per API guidelines

	case "pandascore":
		fetcher = store.NewPandaScoreFetcher(cfg.PandaScore.APIURL, os.Getenv("PANDASCORE_API_KEY"), cfg.PandaScore.SeriesID, cfg.PandaScore.TournamentID)
		limiter = rate.NewLimiter(rate.Every(4*time.Second), 5) // ~900/hr, less than the 1000 limit of our api plan

	default:
		return nil, fmt.Errorf("unsupported data source: %s", cfg.DataSource)
	}

	// Tag each layer's logger with its own component before storing, so that
	// every log line carries exactly one "component" field without double-stamping.
	var appLog, storeLog *slog.Logger
	if log != nil {
		appLog = log.With("component", "app")
		storeLog = log.With("component", "store")
	}
	s, err := store.NewStore(cfg.TournamentName, mongoURI, cfg.Round, fetcher, storeLog)
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
// Returns false if the limiter is nil or the limit has been reached.
// Required since we are treating it as a singleton
func (a *App) Allow() bool {
	if a.rateLimiter == nil {
		return false
	}
	return a.rateLimiter.Allow()
}

// SetUserPrediction contains the logic to set a user prediction in the DB.
// It receives a user struct that contains userID and userName, and a list of teams the user wishes to set,
// and strings containing dbName, collName and round.
// It updates the user's predictions in the database, or returns an error if it occurs.
func (a *App) SetUserPrediction(user models.User, inputTeams []string, round string) error {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return err
	}

	// Get valid team names
	validTeams, formatName, err := a.Store.GetValidTeams()
	if err != nil {
		return err
	}

	f, err := tournament.Get(formatName)
	if err != nil {
		return fmt.Errorf("unknown tournament format: %s", formatName)
	}

	// Get number of required teams
	requiredPredictions := f.RequiredPredictions(len(validTeams))

	// Check num required teams is correct
	if len(inputTeams) != requiredPredictions {
		return fmt.Errorf("incorrect number of teams arguments, expected %d but got %d", requiredPredictions, len(inputTeams))
	}

	// Fix formatting on input teams
	for i := range inputTeams {
		inputTeams[i] = strings.ReplaceAll(inputTeams[i], "\"", "")
		inputTeams[i] = strings.ReplaceAll(inputTeams[i], "“", "")
		inputTeams[i] = strings.ReplaceAll(inputTeams[i], "”", "")
	}

	// Validate input teams
	teams, invalidTeams := scoring.CheckTeamNames(inputTeams, validTeams)
	if len(invalidTeams) > 0 {
		var str strings.Builder
		str.WriteString("the following team names are invalid:")
		for i := range invalidTeams {
			str.WriteString(fmt.Sprintf(" '%s'", invalidTeams[i]))
		}
		return errors.New(str.String())
	}

	// Check for unique team names
	seen := make(map[string]bool)
	for _, team := range teams {
		if seen[team] {
			return fmt.Errorf("'%s' entered multiple times, stored prediction was not updated", team)
		}
		seen[team] = true
	}

	// Generate prediction struct
	prediction, err := f.GeneratePrediction(user, round, teams)
	if err != nil {
		return err
	}

	// Insert prediction to db
	err = a.Store.StoreUserPrediction(user.UserID, prediction)
	if err != nil {
		return err
	}

	return nil
}

// CheckPrediction contains the logic required to check a prediction.
// It receives a user struct and receiver pointer to api.
// It returns a ScoreReport containing the results of the user's predictions, or an error if it occurs.
func (a *App) CheckPrediction(user models.User) (tournament.ScoreReport, error) {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return nil, err
	}
	// Fetch prediction from db
	doc, err := a.Store.GetUserPrediction(user.UserID)
	if err != nil {
		return nil, err
	}

	// Fetch match results from db
	results, err := a.Store.GetMatchResults()
	if err != nil {
		return nil, err
	}

	// Evaluate scores
	report, err := scoring.CalculateUserScore(doc, results)
	if err != nil {
		return nil, err
	}
	return report, nil
}

// CheckPredictionByUsername looks up picks by username (case-insensitive) and scores them.
func (a *App) CheckPredictionByUsername(username string) (models.User, tournament.ScoreReport, error) {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return models.User{}, nil, err
	}
	doc, err := a.Store.GetUserPredictionByUsername(username)
	if err != nil {
		return models.User{}, nil, err
	}
	results, err := a.Store.GetMatchResults()
	if err != nil {
		return models.User{}, nil, err
	}
	report, err := scoring.CalculateUserScore(doc, results)
	if err != nil {
		return models.User{}, nil, err
	}
	return models.User{UserID: doc.UserID, Username: doc.Username}, report, nil
}

// GenerateLeaderboard contains the logic required to generate a leaderboard.
// Preconditions: Receives receiver pointer to api
// Postconditions: Generates the leaderboard, updates it in the DB and returns nil, or returns an error if it occurs
func (a *App) GenerateLeaderboard() error {
	timer := prometheus.NewTimer(metrics.LeaderboardDuration)
	defer timer.ObserveDuration()

	// Check if results have been initialised
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return err
	}

	// Fetch match results from db
	results, err := a.Store.GetMatchResults()
	if err != nil {
		return err
	}

	// Fetch all predictions
	preds, err := a.Store.GetAllUserPredictions()
	if err != nil {
		return err
	}

	var leaderboard store.Leaderboard
	leaderboard.Round = a.Store.GetRound()

	// Iterate over each user's predictions, calculate their score and append the leaderboardEntry to the leaderboard object
	for _, pred := range preds {
		var leaderboardEntry store.LeaderboardEntry
		scoreReport, err := scoring.CalculateUserScore(pred, results)
		if err != nil {
			// Skip predictions that can't be scored — most likely a stale entry
			// stored by an older code version or a different format for this round.
			a.logger().Warn("skipping prediction (stale or incompatible format)",
				"user", pred.Username,
				"round", pred.Round,
				"error", err,
			)
			continue
		}
		scores := scoreReport.GetScore()

		leaderboardEntry.UserID = pred.UserID
		leaderboardEntry.Username = pred.Username
		leaderboardEntry.Score = scores.Successes + scores.Pending - scores.Failed
		leaderboardEntry.ScoreResult.Successes = scores.Successes
		leaderboardEntry.ScoreResult.Pending = scores.Pending
		leaderboardEntry.ScoreResult.Failed = scores.Failed

		leaderboard.Entries = append(leaderboard.Entries, leaderboardEntry)
	}

	err = a.Store.StoreLeaderboard(leaderboard)
	if err != nil {
		return err
	}
	return nil
}

// GetLeaderboard fetches the leaderboard from the db and generates a response string
// Preconditions: Receives receiver pointer to api
// Postconditions: Returns a string with the summary of the leaderboard for this round of the tournament
func (a *App) GetLeaderboard() ([]LeaderboardUser, error) {
	// Fetch leaderboard from DB
	entries, err := a.Store.FetchLeaderboardFromDB()
	if err != nil {
		return nil, err
	}

	// Order the leaderboard in descending order so that the user with the highest score appear at the top. Note score = successes - failures and there is no tie breaker
	sort.Slice(entries, func(i, j int) bool {
		return (entries[i].Score) > (entries[j].Score)
	})

	// Generate Response string
	response := make([]LeaderboardUser, 0, len(entries))
	for i, user := range entries {
		entry := LeaderboardUser{
			Username:  user.Username,
			Rank:      i + 1,
			Successes: user.ScoreResult.Successes,
			Failures:  user.ScoreResult.Failed,
		}
		response = append(response, entry)
	}

	return response, nil
}

// GetTeams gets a list of all valid team names.
// The valid teams list must be initialized in db.
// It returns a string slice containing all valid teams for this round.
func (a *App) GetTeams() ([]Team, error) {
	// Get valid team names
	validTeams, _, err := a.Store.GetValidTeams()
	if err != nil {
		return nil, err
	}

	VRSEntries, err := a.Store.FetchVrsDataFromDB()
	if err != nil {
		return nil, err
	}

	// Build a normalised map for lookup only — original names are never modified.
	// Also keep a slice of normalised keys for fuzzy fallback.
	vrsNorm := make(map[string]int, len(VRSEntries))
	vrsNormKeys := make([]string, 0, len(VRSEntries))
	for _, entry := range VRSEntries {
		key := sources.NormalizeTeamName(entry.TeamName)
		vrsNorm[key] = entry.Standing
		vrsNormKeys = append(vrsNormKeys, key)
	}

	var result []Team
	for _, teamName := range validTeams {
		norm := sources.NormalizeTeamName(teamName)
		ranking, ok := vrsNorm[norm]
		if !ok {
			// Normalised exact match failed — spacing/punctuation difference; try fuzzy
			if matches := fuzzy.RankFind(norm, vrsNormKeys); len(matches) > 0 {
				ranking = vrsNorm[matches[0].Target]
			}
		}
		result = append(result, Team{
			Name:       teamName,
			VRSRanking: ranking,
		})
	}

	return result, nil
}

// GetTeam returns the VRS information for a given team
func (a *App) GetTeam(teamName string) (store.VRSEntry, error) {
	if teamName == "" {
		return store.VRSEntry{}, fmt.Errorf("cannot lookup empty team name")
	}

	VRSEntries, err := a.Store.FetchVrsDataFromDB()
	if err != nil {
		return store.VRSEntry{}, err
	}

	norm := sources.NormalizeTeamName(teamName)
	for _, entry := range VRSEntries {
		if sources.NormalizeTeamName(entry.TeamName) == norm {
			return entry, nil
		}
	}

	// Exact normalised match failed — try fuzzy
	keys := make([]string, len(VRSEntries))
	for i, e := range VRSEntries {
		keys[i] = sources.NormalizeTeamName(e.TeamName)
	}
	if matches := fuzzy.RankFind(norm, keys); len(matches) > 0 {
		for _, entry := range VRSEntries {
			if sources.NormalizeTeamName(entry.TeamName) == matches[0].Target {
				return entry, nil
			}
		}
	}

	return store.VRSEntry{}, fmt.Errorf("no VRS data found for %q", teamName)
}

// GetUpcomingMatches gets the upcoming matches for this round of the tournament. Will only follow the correct path if the scheduled matches data has been initialized.
func (a *App) GetUpcomingMatches() ([]sources.ScheduledMatch, error) {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return nil, err
	}

	scheduledMatches, err := a.Store.FetchMatchSchedule()
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	var matches []sources.ScheduledMatch
	for _, match := range scheduledMatches {
		if match.Finished {
			continue
		}
		// A match whose start time has passed but isn't finished is live.
		// Sources that provide explicit status (e.g. PandaScore "running") set Live directly;
		// for sources without a live flag we infer it from the clock.
		if match.EpochTime < now {
			match.Live = true
		}
		matches = append(matches, match)
	}
	return matches, nil
}

// GetTournamentInfo gets the following information about the tournament: Tournament Name, Round, Format, RequiredPredictions.
// It returns a string slice with the contents attribute : value containing the information listed above.
func (a *App) GetTournamentInfo() (TournamentInfo, error) {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return TournamentInfo{}, err
	}

	// Get valid team names
	validTeams, formatName, err := a.Store.GetValidTeams()
	if err != nil {
		return TournamentInfo{}, err
	}

	f, err := tournament.Get(tournament.Kind(formatName))
	if err != nil {
		return TournamentInfo{}, err
	}
	requiredPredictions := f.RequiredPredictions(len(validTeams))

	return TournamentInfo{
		TournamentName: a.Store.GetDatabase().Name(),
		Round:          a.Store.GetRound(),
		Format:         string(formatName),
		NumTeams:       requiredPredictions,
	}, nil
}

// PopulateMatches fetches and stores the match schedule via the configured data source.
// When scheduleOnly is false, match results are also fetched and stored.
func (a *App) PopulateMatches(scheduleOnly bool) error {
	if !a.Allow() {
		return fmt.Errorf("rate limiter limit reached")
	}

	if err := a.Store.FetchAndStoreSchedule(); err != nil {
		return err
	}

	if !scheduleOnly {
		if err := a.Store.FetchAndUpdateMatchResults(); err != nil {
			return err
		}
	}
	return nil
}

// UpdateMatchSchedule is a rate-limited wrapper around Store.FetchAndStoreSchedule.
func (a *App) UpdateMatchSchedule() error {
	if !a.Allow() {
		return fmt.Errorf("rate limiter exceeded, skipping match schedule update")
	}
	return a.Store.FetchAndStoreSchedule()
}

// StoreSchedule persists a pre-fetched schedule slice. Used by the PandaScore poller
// to reuse already-fetched data instead of making a second API call.
func (a *App) StoreSchedule(matches []sources.ScheduledMatch) error {
	return a.Store.StoreMatchSchedule(matches)
}

// UpdateMatchResults is a wrapper function for App.Store.FetchAndUpdateMatchResults() that enforces rate limiting
// across the app and ensuring we comply with api specifications
func (a *App) UpdateMatchResults() error {
	if !a.Allow() {
		return fmt.Errorf("rate limiter exceeded, skipping match result update")
	}

	if err := a.Store.FetchAndUpdateMatchResults(); err != nil {
		return err
	}
	metrics.MatchUpdatesTotal.Inc()
	return nil
}
