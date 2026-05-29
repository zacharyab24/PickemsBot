package store

import (
	"context"
	"pickems-bot/metrics"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// VRSEntry represents a single team's entry in the VRS world rankings database.
type VRSEntry struct {
	Standing      int       `bson:"standing"`
	Points        int       `bson:"points"`
	TeamName      string    `bson:"team_name"`
	Roster        []string  `bson:"roster"`
	StandingsDate string    `bson:"standings_date"`
	SyncedAt      time.Time `bson:"synced_at"`
}

// FetchVrsDataFromDB retrieves all entries from the VRS rankings collection.
// Note: fetches the entire collection on each call. This is acceptable given the
// expected volume (~300 documents), but should be revisited if that changes.
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
