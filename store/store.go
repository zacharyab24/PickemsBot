/* store.go
 * Contains the store struct and NewStore function. The methods for this package were split into three files:
 * match_results, upcoming_matches and user_predictions. Each of these files contain methods for interacting with that
 * part of the database
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"log/slog"

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
	log     *slog.Logger
}

// logger returns the store's logger, falling back to the global default when none was injected.
func (s *Store) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}

// NewStore initializes Store. Sets global values and initialises db connection.
// log may be nil; if so the global slog default is used.
func NewStore(dbName string, mongoURI string, round string, fetcher DataSourceFetcher, log *slog.Logger) (*Store, error) {
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
		log:     log,
	}, nil
}

var _ Interface = (*Store)(nil)
