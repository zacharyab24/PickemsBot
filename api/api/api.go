/* api.go
 * This file contains the public methods for interacting with this package. For consistent results, fuctions should
 * only be called from this file, not the sub packages for match and processing. For details about functionality see `api.md`
 * Authors: Zachary Bower
 */

package api

import (
	"errors"
	"fmt"
	"os"
	"pickems-bot/api/external"
	"pickems-bot/api/logic"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

// API provides methods for interacting with the pickems bot data layer
type API struct {
	Store store.Interface
}

// NewAPI creates a new API instance with the provided configuration
func NewAPI(dbName string, mongoURI string, page string, params string, round string) (*API, error) {
	if dbName == "" || page == "" || round == "" {
		return nil, fmt.Errorf("dbName, page, and round are required")
	}
	// Append round to page string
	//page = fmt.Sprintf("%s/%s", page, round)

	s, err := store.NewStore(dbName, mongoURI, page, params, round)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	return &API{
		Store: s,
	}, nil
}

// SetUserPrediction contains the logic to set a user prediction in the DB.
// It receives a user struct that contains userID and userName, and a list of teams the user wishes to set,
// and strings containing dbName, collName and round.
// It updates the user's predictions in the database, or returns an error if it occurs.
func (a *API) SetUserPrediction(user shared.User, inputTeams []string, round string) error {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return err
	}

	// Get valid team names
	validTeams, format, err := a.Store.GetValidTeams()
	if err != nil {
		return err
	}

	// Get number of required teams
	var requiredPredictions int
	switch format {
	case "swiss":
		requiredPredictions = 10
	case "single-elimination":
		T := len(validTeams)
		requiredPredictions = T / 2
	default:
		return fmt.Errorf("unknown tournament format: %s", format)
	}

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
	teams, invalidTeams := logic.CheckTeamNames(inputTeams, validTeams)
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
	prediction, err := logic.GeneratePrediction(user, format, round, teams, requiredPredictions)
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
// It returns a string containing the results of the user's predictions, or an error if it occurs.
func (a *API) CheckPrediction(user shared.User) (string, error) {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return "", err
	}
	// Fetch prediction from db
	doc, err := a.Store.GetUserPrediction(user.UserID)
	if err != nil {
		return "", err
	}

	// Fetch match results from db
	results, err := a.Store.GetMatchResults()
	if err != nil {
		return "", err
	}

	// Evaluate scores
	_, report, err := logic.CalculateUserScore(doc, results)
	if err != nil {
		return "", err
	}
	return report, nil
}

// GetLeaderboard contains the logic required to get the leaderboard results.
// It receives receiver pointer to api and returns a string containing the leaderboard for the tournament.
func (a *API) GetLeaderboard() (string, error) {
	// Check if results have been initialised
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return "", err
	}

	// Fetch match results from db
	results, err := a.Store.GetMatchResults()
	if err != nil {
		return "", err
	}

	// Fetch all predictions
	preds, err := a.Store.GetAllUserPredictions()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "There are no user predictions currently stored", nil
		}
		return "", err
	}

	var leaderboard []LeaderboardEntry

	// Iterate over each user's predictions and calculate their score
	for _, pred := range preds {
		scores, _, err := logic.CalculateUserScore(pred, results)
		if err != nil {
			return "", err
		}
		leaderboard = append(leaderboard, LeaderboardEntry{Username: pred.Username, Succeeded: scores.Successes, Failed: scores.Failed})
	}

	// Order the leaderboard in decesending order so that the user with the highest score appear at the top. Note score = successes - failures and there is no tie breaker
	sort.Slice(leaderboard, func(i, j int) bool {
		return (leaderboard[i].Succeeded - leaderboard[i].Failed) > (leaderboard[j].Succeeded - leaderboard[j].Failed)
	})

	// Generate Responnse stirng
	var response strings.Builder
	response.WriteString("The users with the best pickems are:\n")
	for i, user := range leaderboard {
		response.WriteString(fmt.Sprintf("%d. %s, %d successes, %d failures\n", i+1, user.Username, user.Succeeded, user.Failed))
	}

	return response.String(), nil
}

// GetTeams gets a list of all valid team names.
// The valid teams list must be initialized in db.
// It returns a string slice containing all valid teams for this round.
func (a *API) GetTeams() ([]string, error) {
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

// GetUpcomingMatches gets the upcoming matches for this round of the tournament.
// It receives receiver pointer to api. Will only follow the correct path if the scheduled matches data has been initialized.
// It returns a string slice containing all upcoming matches in this round.
func (a *API) GetUpcomingMatches() ([]string, error) {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return nil, err
	}

	scheduledMatches, err := a.Store.FetchMatchSchedule()
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, match := range scheduledMatches {
		// upcomingMatches contains all matches for a round in tournament, not just future ones, however in this function
		// we only care about the ones in the future, so if the start time is before now, don't add it to the response
		// we can't rely on []external.ScheduledMatch as this only gets the data whenever PopulateScheduledMatches is run
		if match.EpochTime < time.Now().Unix() || match.Finished {
			continue
		}
		streamURL := getTwitchURL(match.StreamURL)
		if streamURL == "unknown" {
			matches = append(matches, fmt.Sprintf("- %s VS %s (bo%s): <t:%d>\n", match.Team1, match.Team2, match.BestOf, match.EpochTime))
		} else {
			matches = append(matches, fmt.Sprintf("- %s VS %s (bo%s): <t:%d>: %s\n", match.Team1, match.Team2, match.BestOf, match.EpochTime, streamURL))
		}
	}
	return matches, nil
}

// GetTournamentInfo gets the following information about the tournament: Tournament Name, Round, Format, RequiredPredictions.
// It returns a string slice with the contents attribute : value containing the information listed above.
func (a *API) GetTournamentInfo() ([]string, error) {
	err := a.Store.EnsureScheduledMatches()
	if err != nil {
		return nil, err
	}

	// Get valid team names
	validTeams, format, err := a.Store.GetValidTeams()
	if err != nil {
		return nil, err
	}

	// Get number of required teams
	var requiredPredictions int
	switch format {
	case "swiss":
		requiredPredictions = 10
	case "single-elimination":
		T := len(validTeams)
		requiredPredictions = T / 2
	default:
		requiredPredictions = 0
	}

	var values []string
	values = append(values, fmt.Sprintf("Tournament Name: %s", a.Store.GetDatabase().Name()))
	values = append(values, fmt.Sprintf("Round: %s", a.Store.GetRound()))
	values = append(values, fmt.Sprintf("Format: %s", format))
	values = append(values, fmt.Sprintf("Number of required teams: %d", requiredPredictions))
	return values, nil
}

// PopulateMatches fetches scheduled match data and stores it in the DB. Needs to be run before other functions in this package will work properly.
// It receives receiver pointer to API and returns nil, or an error if it occurs.
func (a *API) PopulateMatches(scheduleOnly bool) error {
	// Populated Scheduled matches
	scheduledMatches, err := external.FetchScheduledMatches(os.Getenv("LIQUIDPEDIADB_API_KEY"), a.Store.GetPage(), a.Store.GetOptionalParams())
	if err != nil {
		return err
	}

	// Populate Scheduled Matches
	err = a.Store.StoreMatchSchedule(scheduledMatches)
	if err != nil {
		return err
	}

	if !scheduleOnly { // Only run if scheduleOnly is false, this way we can store upcoming matches of unsupported match structures
		// Populate Match Results -> due to some spaghetti code, this also populates match schedule
		_, err = a.Store.GetMatchResults()
		if err != nil {
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
