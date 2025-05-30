/* user_predictions.go
 * Contains the methods for interacting with the user_predictions collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Function to store user predictions in the db
// Preconditions: receives strings containing db name, collection name and userId, and Prediction containing the users predictions
// Postconditions: stores or updates the user's prediction stored in the db, or returns an error if the opperations was unsuccessful
func (s *Store) StoreUserPrediction(userId string, userPrediction Prediction) error {
	// Attempt to find an existing document
	var result Prediction
	err := s.Collections.Predictions.FindOne(context.TODO(), bson.M{"userid": userId, "round": userPrediction.Round}).Decode(&result)
	notFound := err == mongo.ErrNoDocuments

	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing prediction failed: %w", err)
	}

	update := bson.M {
		"$set": userPrediction,
	}
	filter := bson.M{
		"userid": userId,
		"round": userPrediction.Round,
	}

	// The user currently does not have predictions stored so we create a new document
	if notFound {
		_, err := s.Collections.Predictions.InsertOne(context.TODO(), userPrediction)
		if err != nil {
			return fmt.Errorf("failed to insert new user prediction: %w", err)
		}
		return nil
	}

	// Else update the user's existing prediction
	_, err = s.Collections.Predictions.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update existing user prediction: %w", err)
	}
	return nil
}

// Function to do DB lookup and get prediction for a user
// Preconditions: receives strings containing db name, collection name and userId
// Postconditions: returns a user's prediction if it exists, or an error if it occurs
func (s *Store) GetUserPrediction(userId string) (Prediction, error) {
	opts := options.FindOne()

	var result Prediction
	err := s.Collections.Predictions.FindOne(context.TODO(), bson.M{"userid": userId, "round": s.Round}, opts).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Prediction{}, err
		}
		return Prediction{}, fmt.Errorf("error fetching results from db: %w", err)
	}

	return result, nil
}

// Function to do DB lookup and the predictions for all users with predictions stored for a round. Used in leaderboard calculations
// Preconditions: Receives strings containing database name, colletionname and round
// Postconditions: Returns slice of Predictions or an error if it occurs
func (s *Store) GetAllUserPredictions() ([]Prediction, error) {
	// Filter query to match documents where the round is the round sting input to the function
	filter := bson.D{{Key: "round", Value: s.Round}}
	
	// Retrieves documents that match the filter
	cursor, err := s.Collections.Predictions.Find(context.TODO(), filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		return nil, fmt.Errorf("error fetching results from db: %w", err)
	}
	
	// Unpack the cursor into a slice
	var results []Prediction
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, fmt.Errorf("error unpacking cursor into slice of predictions: %w", err)
	}
	
	return results, nil
}

// Helper function to get valid team names used in setting user predictions. We are going to grab the valid team names
// from the results table as this already contains a list of names, and lets us filter by round without needing to
// create and maintain a new collection that will require more api calls
// Preconditions: Receives db name, collection name and round strings
// Postconditions: Returns string slice containing valid team names for the round, or returns error if an issue occurs
func (s *Store) GetValidTeams() ([]string, string, error) {
	// Get results stored in our db
	dbResults, err := s.FetchMatchResultsFromDb()
	if err != nil {
		return nil, "", err
	}

 	var teamNames []string

    // Type assertion to determine the concrete type and extract team names
    switch result := dbResults.(type) {
    case SwissResultRecord:
        // For Swiss format, Teams is map[string]string
        for teamName := range result.Teams {
			teamNames = append(teamNames, teamName)
        }
    case EliminationResultRecord:
        // For Elimination format, Progression is map[string]*TeamProgress
        for teamName := range result.Teams {
            teamNames = append(teamNames, teamName)
        }
    default:
        return nil, "", fmt.Errorf("unknown result record type: %T", result)
    }

    return teamNames,dbResults.GetType(), nil
}