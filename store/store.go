/* store.go
 * Contains the store struct and NewStore function. The methods for this package were split into three files:
 * match_results, upcoming_matches and user_predictions. Each of these files contain methods for interacting with that part of the database
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
	Client             *mongo.Client
	TournamentDatabase *mongo.Database
	VRSDatabase        *mongo.Database
	Round              string
	Collections        Collections
	Fetcher            DataSourceFetcher
	log                *slog.Logger
}

// Collections represents all of the structs the store object calls
// All except VRS are part of this tournament's definited database
// VRS lives in a seperate database, on the same mongo server
type Collections struct {
	Predictions   *mongo.Collection
	MatchResults  *mongo.Collection
	MatchNodes    *mongo.Collection
	MatchSchedule *mongo.Collection
	Leaderboard   *mongo.Collection
	VRS           *mongo.Collection
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
	vrsDb := client.Database("VRS_RANKINGS") // Hardcoded name for now, exists on the db server independent of tournament, and is updated externally

	return &Store{
		Client:             client,
		TournamentDatabase: db,
		VRSDatabase:        vrsDb,
		Round:              round,
		Collections: Collections{
			Predictions:   db.Collection("user_predictions"),
			MatchResults:  db.Collection("match_results"),
			MatchNodes:    db.Collection("match_nodes"),
			MatchSchedule: db.Collection("scheduled_matches"),
			Leaderboard:   db.Collection("leaderboard"),
			VRS:           vrsDb.Collection("2026"),
		},
		Fetcher: fetcher,
		log:     log,
	}, nil
}

var _ Interface = (*Store)(nil)
