/* store.go
 * Contains the store struct and NewStore function. The methods for this package were split into three files:
 * match_results, upcoming_matches and user_predictions. Each of these files contain methods for interacting with that
 * part of the database
 * Authors: Zachary Bower
 */

package store

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store represents the database connection and configuration
type Store struct {
	Client      *mongo.Client
	Database    *mongo.Database
	Round       string
	Collections struct {
		Predictions   *mongo.Collection
		MatchResults  *mongo.Collection
		MatchNodes    *mongo.Collection
		MatchSchedule *mongo.Collection
		Leaderboard   *mongo.Collection
	}
	Fetcher DataSourceFetcher
}

// NewStore initializes Store. Sets global values and initialises db connection
func NewStore(dbName string, mongoURI string, round string, fetcher DataSourceFetcher) (*Store, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}
	db := client.Database(dbName)

	return &Store{
		Client:   client,
		Database: db,
		Round:    round,
		Collections: struct {
			Predictions   *mongo.Collection
			MatchResults  *mongo.Collection
			MatchNodes    *mongo.Collection
			MatchSchedule *mongo.Collection
			Leaderboard   *mongo.Collection
		}{
			Predictions:   db.Collection("user_predictions"),
			MatchResults:  db.Collection("match_results"),
			MatchNodes:    db.Collection("match_nodes"),
			MatchSchedule: db.Collection("scheduled_matches"),
			Leaderboard:   db.Collection("leaderboard"),
		},
		Fetcher: fetcher,
	}, nil
}

var _ Interface = (*Store)(nil)
