/* leaderboard.go
 * Contains the methods for interacting with the leaderboard collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// FetchLeaderboardFromDB returns the leaderboard from the db
// Preconditions: Receives receiver pointer for Store which contains DB information such as database name, collection and round
// Postconditions: Returns slice of LeaderboardEntry with user data, or an error if it occurs
func (s *Store) FetchLeaderboardFromDB() ([]LeaderboardEntry, error) {
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

// StoreLeaderboard updates the leaderboard stored in the DB
// Preconditions: Receives receiver pointer for Store and the Leaderboard value to be stored
// Postconditions: Updates the leaderboard collection in Mongo and returns nil, or an error if it occurs
func (s *Store) StoreLeaderboard(leaderboard Leaderboard) error {
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
	log.Println("updating leaderboard in db")
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
