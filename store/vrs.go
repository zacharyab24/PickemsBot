package store

import (
	"context"
	"pickems-bot/metrics"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type VRSEntry struct {
	Standing      int       `bson:"standing"`
	Points        int       `bson:"points"`
	TeamName      string    `bson:"team_name"`
	Roster        []string  `bson:"roster"`
	StandingsDate string    `bson:"standings_date"`
	SyncedAt      time.Time `bson:"synced_at"`
}

// fetchVrsDataFromDB gets all results form the VRS database and returns them as a slice
// Note this function is not efficient, it grabs everything from the db
// However, there **should** only be ~300 documents, each with 11 fields, so it isn't a concern for now
// Adding some safety around this is a future enhancement
func (s *Store) FetchVrsDataFromDB() ([]VRSEntry, error) {
	metrics.MongoOpsTotal.WithLabelValues("read").Inc()
	var results []VRSEntry
	cursor, err := s.Collections.VRS.Find(context.TODO(), bson.D{})
	if err != nil {
		return nil, err
	}

	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, err
	}
	return results, err
}
