/* api.go
 * This file contains the public methods for interacting with this package. For consistent results, fuctions should
 * only be called from this file, not the sub packages for match and processing. For details about functionality see `api.md`
 * Authors: Zachary Bower
 */

package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"pickems-bot/models"
	"pickems-bot/scoring"
	"pickems-bot/sources"
	"pickems-bot/store"
	"pickems-bot/tournament"
	"sort"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// App provides methods for interacting with the pickems bot data layer
type App struct {
	Store       store.Interface
	rateLimiter *rate.Limiter
}

// NewAPI creates a new App instance with the provided configuration
func NewAPI(dbName string, mongoURI string, page string, format string, round string) (*App, error) {
	if dbName == "" || page == "" || round == "" {
		return nil, fmt.Errorf("dbName, page, and round are required")
	}

	s, err := store.NewStore(dbName, mongoURI, page, format, round)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	return &App{
		Store:       s,
		rateLimiter: rate.NewLimiter(rate.Every(time.Minute), 10), // Rate limit liquipedia calls to 60 per hour as per api guidelines
	}, nil
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

// GenerateLeaderboard contains the logic required to generate a leaderboard.
// Preconditions: Receives receiver pointer to api
// Postconditions: Generates the leaderboard, updates it in the DB and returns nil, or returns an error if it occurs
func (a *App) GenerateLeaderboard() error {
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
			log.Printf("warning: skipping prediction for user %s (round=%s): %v", pred.Username, pred.Round, err)
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
func (a *App) GetTeams() ([]string, error) {
	// We need to ensure scheduled match data exists, as this function relies on the results data being populated, which needs scheduled matches. Theres some pretty bad nesting / dependencies in this code base
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return nil, err
	}

	// Get valid team names
	validTeams, _, err := a.Store.GetValidTeams()
	if err != nil {
		return nil, err
	}

	return validTeams, nil
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

	var matches []sources.ScheduledMatch
	for _, match := range scheduledMatches {
		// upcomingMatches contains all matches for a round in tournament, not just future ones, however in this function
		// we only care about the ones in the future, so if the start time is before now, don't add it to the response
		// we can't rely on []sources.ScheduledMatch as this only gets the data whenever PopulateScheduledMatches is run
		if match.EpochTime < time.Now().Unix() || match.Finished {
			continue
		}
		// Pre-process stream URL into a full link the handler can use directly
		url := getTwitchURL(match.StreamURL)
		if url == "unknown" {
			match.StreamURL = ""
		} else {
			match.StreamURL = url
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

// PopulateMatches fetches all match data for the tournament page in a single
// LiquipediaDB call and stores both the match schedule and (when scheduleOnly
// is false) the match results. Using one API call avoids a duplicate request
// that would otherwise be made by the separate schedule and result fetchers.
// Preconditions: Receives receiver pointer to App and returns nil, or an error if it occurs.
func (a *App) PopulateMatches(scheduleOnly bool) error {
	if a.rateLimiter == nil {
		return fmt.Errorf("rate limiter not initialised")
	}
	if !a.rateLimiter.Allow() {
		log.Printf("Rate limit exceeded")
		return fmt.Errorf("rate limiter limit reached")
	}

	jsonResponse, err := sources.GetLiquipediaMatchDataByPage(os.Getenv("LIQUIDPEDIADB_API_KEY"), a.Store.GetPage())
	if err != nil {
		return fmt.Errorf("error fetching match data from liquipedia api: %w", err)
	}

	scheduledMatches, err := sources.ParseLiquipediaSchedule(jsonResponse)
	if err != nil {
		return err
	}
	sort.Slice(scheduledMatches, func(i, j int) bool {
		return scheduledMatches[i].EpochTime < scheduledMatches[j].EpochTime
	})
	if err = a.Store.StoreMatchSchedule(scheduledMatches); err != nil {
		return err
	}

	if !scheduleOnly {
		if err = a.Store.FetchAndUpdateMatchResultsFromJSON(jsonResponse); err != nil {
			return err
		}
	}
	return nil
}

// getTwitchURL is a helper function to get the twitch url from the liquipedia stream url.
// It receives a string containing stream name and returns the correct steam name or unknown if it is not in the hard coded list of steam names.
func getTwitchURL(streamURL string) string {
	urls := make(map[string]string)
	urls["BLAST_Premier"] = "https://www.twitch.tv/blastpremier"
	urls["BLAST"] = "https://www.twitch.tv/blast"
	// Put more here as needed

	url, ok := urls[streamURL]
	if !ok {
		return "unknown"
	}
	return url
}

// UpdateMatchResults is a wrapper function for App.Store.FetchAndUpdateMatchResults() that enforces rate limiting
// across the app and ensuring we comply with LiquipediaDB api specifications
// Preconditions: Receives receiver pointer for api
// Postconditions: Updates the match results database, or throws an error if rate limit has been reached or other error occurs
func (a *App) UpdateMatchResults() error {
	if a.rateLimiter == nil {
		return fmt.Errorf("rate limiter not initialised")
	}

	if !a.rateLimiter.Allow() {
		log.Println("Rate limiter is reached")
		return fmt.Errorf("rate limiter exceeded, skipping match result update")
	}

	return a.Store.FetchAndUpdateMatchResults()
}
