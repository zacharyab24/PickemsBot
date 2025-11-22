/* store.go
 * Contains the store struct and NewStore function. The methods for this package were split into three files:
 * match_results, upcoming_matches and user_predictions. Each of these files contain methods for interacting with that
 * part of the database
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	Client         *mongo.Client
	Database       *mongo.Database
	Page           string
	OptionalParams string
	Round          string
	Collections    struct {
		Predictions   *mongo.Collection
		MatchResults  *mongo.Collection
		MatchSchedule *mongo.Collection
	}
}

// Function for initialsing Store. Sets global values and initialises db connection
// Preconditions: Receives strings containing the following: dbName, mongoURI, page, params and round
// Postconditions: Updates global values, sets collection values, and returns pointer to the Store object, or error if it occurs
func NewStore(dbName string, mongoURI string, page string, params string, round string) (*Store, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}
	db := client.Database(dbName)

	if page == "" || round == "" {
		return nil, fmt.Errorf("page or round cannot be empty")
	}

	return &Store{
		Client:         client,
		Database:       db,
		Page:           page,
		OptionalParams: params,
		Round:          round,
		Collections: struct {
			Predictions   *mongo.Collection
			MatchResults  *mongo.Collection
			MatchSchedule *mongo.Collection
		}{
			Predictions:   db.Collection("user_predictions"),
			MatchResults:  db.Collection("match_results"),
			MatchSchedule: db.Collection("scheduled_matches"),
		},
	}, nil
}
