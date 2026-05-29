/* leaderboard.go
 * Contains the methods for interacting with the leaderboard collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"pickems-bot/metrics"
	"pickems-bot/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LeaderboardEntry represents a single entry in the leaderboard for a user
type LeaderboardEntry struct {
	UserID             string `bson:"userid,omitempty"`
	Username           string `bson:"username,omitempty"`
	Score              int    `bson:"score,omitempty"`
	models.ScoreResult `bson:",inline"`
}

// Leaderboard represents the tournament leaderboard stored in MongoDB
type Leaderboard struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Round     string             `bson:"round,omitempty"`
	UpdatedAt time.Time          `bson:"updated_at"`
	Entries   []LeaderboardEntry `bson:"entries"`
}

// FetchLeaderboardFromDB returns the leaderboard entries for the current round.
func (s *Store) FetchLeaderboardFromDB() ([]LeaderboardEntry, error) {
	metrics.MongoOpsTotal.WithLabelValues("read").Inc()
	s.Collections.Leaderboard.Name()
	opts := options.FindOne()

	var res Leaderboard
	err := s.Collections.Leaderboard.FindOne(context.TODO(), bson.D{{Key: "round", Value: s.Round}}, opts).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch leaderboard from database: %w", err)
	}

	return res.Entries, nil
}

// StoreLeaderboard persists the given leaderboard, inserting a new document or replacing an existing one for the current round.
func (s *Store) StoreLeaderboard(leaderboard Leaderboard) error {
	metrics.MongoOpsTotal.WithLabelValues("write").Inc()
	if reflect.DeepEqual(leaderboard, Leaderboard{}) {
		return fmt.Errorf("leaderboard is empty")
	}

	// Attempt to find an existing document
	var res Leaderboard
	err := s.Collections.Leaderboard.FindOne(context.TODO(), bson.D{{Key: "round", Value: s.Round}}).Decode(&res)
	notFound := err == mongo.ErrNoDocuments

	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing record failed: %w", err)
	}

	// Perform insert or update
	s.logger().Info("updating leaderboard in db", "round", s.Round)
	if notFound {
		_, err := s.Collections.Leaderboard.InsertOne(context.TODO(), leaderboard)
		if err != nil {
			return fmt.Errorf("leaderboard insert failed: %w", err)
		}
		return nil
	}

	filter := bson.M{"round": s.Round}
	update := bson.D{{Key: "$set", Value: leaderboard}}

	_, err = s.Collections.Leaderboard.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("leaderboard update failed: %w", err)
	}
	return nil
}
