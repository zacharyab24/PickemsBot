/* api.go
 * This file contains the public methods for interacting with this package. For consistent results, fuctions should
 * only be called from this file, not the sub packages for match and processing. For details about functionality see `api.md`
 * Authors: Zachary Bower
 */

package api

import (
	"errors"
	"fmt"
	"pickems-bot/api/logic"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
	"strings"
)

type API struct {
	Store *store.Store
}

func NewAPI(dbName string, mongoURI string, page string, params string, round string) (*API, error) {
	if dbName == "" || page == "" {
		return nil, fmt.Errorf("dbName, page, and params are required")
	}

	s, err := store.NewStore(dbName, mongoURI, page, params, round)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	return &API{
		Store:  s,
	}, nil
}

// Function that contains the logic to set a user prediction in the DB
// Preconditions: Receives user struct that contains userId and userName, and a list of teams the user wishes to set, 
// and strings containing dbName, collName and round 
// Postconditions: Updates the user's predictions in the database, or returns an error if it occurs
func (a *API) SetUserPrediction(user shared.User, inputTeams []string, round string) error {
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
	case "single-elimination" :
		T := len(validTeams)
		requiredPredictions = T / 2
	default:
		return fmt.Errorf("unknown tournament format: %s", format)
	}

	// Check num required teams is correct
	if len(inputTeams) != requiredPredictions {
		return fmt.Errorf("incorrect number of teams arguments, expected %d but got %d", requiredPredictions, len(inputTeams))
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

	// Generate prediction struct
	prediction, err := logic.GeneratePrediction(user, format, round, teams, requiredPredictions)
	if err != nil {
		return err
	}

	// Insert prediction to db
	err = a.Store.StoreUserPrediction(user.UserId, prediction)
	if err != nil {
		return err
	}
	
	return nil
}

// Function that contains the logic required to check a prediction
// Preconditions: Receives a user struct
// Postconditions: Returns a string containing the results of the user's predictions, or an error if it occurs
func (a *API) CheckPrediction(user shared.User) (string, error) {
	
	// Fetch prediction from db
	doc, err := a.Store.GetUserPrediction(user.UserId)
	if err != nil {
		fmt.Println("foo")
		return "", err
	}

	// Fetch match results from db
	results, err := a.Store.GetMatchResults()
	if err != nil {
		fmt.Println("bar")
		return "", err
	}

	// Evaluate scores
	_, report, err := logic.CalculateUserScore(doc, results)
	if err != nil {
		fmt.Println("deez")
		return "", err
	}
	return report, nil
}

// Function that contains the logic required to get the leaderboard results
// Preconditions: None
// Postconditions: Returns a string containing the leaderboard for the tournament
func GetLeaderboard() (string, error) {
	return "", nil
}

// Function to get a list of all valid team names
// Preconditions: None
// Postconditions: Returns a string slice containing all valid teams for this round
func GetTeams() ([]string, error) {
	return nil, nil
}

// Function to get the upcoming matches for this round of the tournament
// Preconditions: None
// Postconditions: Returns a string slice containing all upcoming matches in this round
func GetUpcomingMatches() ([]string, error) {
	return nil, nil
}
