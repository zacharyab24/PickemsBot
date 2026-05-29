/* user_predictions.go
 * Contains the methods for interacting with the user_predictions collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"

	"pickems-bot/metrics"
	"pickems-bot/models"
	"pickems-bot/tournament"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StoreUserPrediction inserts a new prediction for the given user, or replaces an existing one for the current round.
func (s *Store) StoreUserPrediction(userID string, userPrediction models.Prediction) error {
	metrics.MongoOpsTotal.WithLabelValues("write").Inc()
	// Attempt to find an existing document
	var result models.Prediction
	err := s.Collections.Predictions.FindOne(context.TODO(), bson.M{"userid": userID, "round": userPrediction.Round}).Decode(&result)
	notFound := errors.Is(err, mongo.ErrNoDocuments)

	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing prediction failed: %w", err)
	}

	update := bson.M{
		"$set": userPrediction,
	}
	filter := bson.M{
		"userid": userID,
		"round":  userPrediction.Round,
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

// GetUserPrediction retrieves the stored prediction for the given user ID in the current round.
func (s *Store) GetUserPrediction(userID string) (models.Prediction, error) {
	metrics.MongoOpsTotal.WithLabelValues("read").Inc()
	opts := options.FindOne()

	var result models.Prediction
	err := s.Collections.Predictions.FindOne(context.TODO(), bson.M{"userid": userID, "round": s.Round}, opts).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.Prediction{}, err
		}
		return models.Prediction{}, fmt.Errorf("error fetching results from db: %w", err)
	}

	return result, nil
}

// GetAllUserPredictions returns all stored predictions for the current round.
func (s *Store) GetAllUserPredictions() ([]models.Prediction, error) {
	metrics.MongoOpsTotal.WithLabelValues("read").Inc()
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
	var results []models.Prediction
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, fmt.Errorf("error unpacking cursor into slice of predictions: %w", err)
	}

	return results, nil
}

// GetValidTeams returns the valid team names and tournament format for the current round, derived from the stored match results.
func (s *Store) GetValidTeams() ([]string, tournament.Kind, error) {
	// Get results stored in our db
	dbResults, err := s.FetchMatchResultsFromDb()
	if err != nil {
		return nil, "", err
	}

	return dbResults.GetTeamNames(), dbResults.GetType(), nil
}
