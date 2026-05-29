/* match_schedule.go
 * Contains the methods for interacting with the match_schedule collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"
	"pickems-bot/metrics"
	"pickems-bot/sources"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UpcomingMatchDoc represents upcoming match data stored in the database
type UpcomingMatchDoc struct {
	Round            string                   `bson:"round,omitempty"`
	ScheduledMatches []sources.ScheduledMatch `bson:"scheduled_matches,omitempty"`
}

// FetchMatchSchedule returns the scheduled matches for the current round from the database.
func (s *Store) FetchMatchSchedule() ([]sources.ScheduledMatch, error) {
	metrics.MongoOpsTotal.WithLabelValues("read").Inc()
	opts := options.FindOne()

	// Get UpcomingMatchDoc result from db
	var res UpcomingMatchDoc
	err := s.Collections.MatchSchedule.FindOne(context.TODO(), bson.D{{Key: "round", Value: s.Round}}, opts).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("error fetching results from db: %w", err)
	}
	return res.ScheduledMatches, nil
}

// StoreMatchSchedule persists a slice of scheduled matches for the current round, inserting a new document or replacing an existing one.
func (s *Store) StoreMatchSchedule(scheduledMatches []sources.ScheduledMatch) error {
	metrics.MongoOpsTotal.WithLabelValues("write").Inc()
	if len(scheduledMatches) == 0 {
		return fmt.Errorf("scheduled matches input has length 0, requires at least 1")
	}

	// Attempt to find an existing document
	var raw bson.M
	err := s.Collections.MatchSchedule.FindOne(context.TODO(), bson.M{"round": s.Round}).Decode(&raw)
	notFound := errors.Is(err, mongo.ErrNoDocuments)

	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing record failed: %w", err)
	}

	// Create bson UpcomingMatchDoc
	filter := bson.M{"round": s.Round}
	upcomingMatchDoc := UpcomingMatchDoc{
		Round:            s.Round,
		ScheduledMatches: scheduledMatches,
	}
	update := bson.M{"$set": upcomingMatchDoc}

	s.logger().Info("updating match schedule in db", "round", s.Round)

	// Perform insert or update
	if notFound {
		_, err := s.Collections.MatchSchedule.InsertOne(context.TODO(), upcomingMatchDoc)
		if err != nil {
			return fmt.Errorf("failed to insert upcoming matches: %w", err)
		}
		return nil
	}
	_, err = s.Collections.MatchSchedule.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update upcoming matches: %w", err)
	}

	return nil
}

// EnsureScheduledMatches verifies that at least one scheduled match exists in the database for the current round.
// Prediction operations depend on this data being present, so callers should use this as a precondition check.
func (s *Store) EnsureScheduledMatches() error {
	var result struct {
		ScheduledMatches []sources.ScheduledMatch `bson:"scheduled_matches"`
	}
	filter := bson.M{"round": s.Round}
	err := s.Collections.MatchSchedule.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("no scheduled matches entry found for round %s", s.Round)
		}
		return fmt.Errorf("error checking scheduled matches: %w", err)
	}

	if len(result.ScheduledMatches) == 0 {
		return fmt.Errorf("scheduled match collection found but its results were empty for round %s", s.Round)
	}
	return nil
}

// FetchAndStoreSchedule fetches upcoming matches from the configured data source and persists them to the database.
func (s *Store) FetchAndStoreSchedule() error {
	matches, err := s.Fetcher.FetchSchedule()
	if err != nil {
		return err
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].EpochTime < matches[j].EpochTime
	})
	return s.StoreMatchSchedule(matches)
}
