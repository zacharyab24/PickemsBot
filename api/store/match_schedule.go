/* match_schedule.go
 * Contains the methods for interacting with the match_schedule collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"pickems-bot/api/external"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Function used to fetch scheduled matches for a round from db
// Preconditions: Receives receiver pointer for Store which contains DB information such as database name, collection and round
// Postconditions: Returns slice of upcoming matches or error message if the operation was unsuccessful
func (s *Store) FetchMatchSchedule() ([]external.ScheduledMatch, error) {
	opts := options.FindOne()

	// Get UpcomingMatchDoc result from db
	var res UpcomingMatchDoc
	var shouldRefresh bool
	err := s.Collections.MatchSchedule.FindOne(context.TODO(), bson.D{{Key: "round", Value: s.Round}}, opts).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			shouldRefresh = true
		}
		return nil, fmt.Errorf("error fetching results from db: %w", err)
	} else if res.TTL < time.Now().Unix() {
		shouldRefresh = true
	}

	// Run if we need to refresh the data stored in the db (either there is no data stored or the TTL has experied)
	if shouldRefresh {
		// get new data from liquipediadb api
		externalResults, err := external.FetchScheduledMatches(os.Getenv("LIQUIDPEDIADB_API_KEY"), s.Page, s.OptionalParams)
		if err != nil {
			return nil, err
		}
		err = s.StoreMatchSchedule(externalResults)
		if err != nil {
			return nil, err
		}
		return externalResults, nil
	}

	return res.ScheduledMatches, nil
}

// Function to store upcoming matches
// Preconditions: Receives pointer for Store which contains DB information such as database name, collection and round
// and slice slice of []external.ScheduledMatch containing the data to be stored
// Postconditions: Updates the data stored in the db, returns error message if the operation was unsuccessful
func (s *Store) StoreMatchSchedule(scheduledMatches []external.ScheduledMatch) error {
	if len(scheduledMatches) == 0 {
		return fmt.Errorf("scheduled matches input has length 0, requires at least 1")
	}

	// Attempt to find an existing document
	var raw bson.M
	err := s.Collections.MatchSchedule.FindOne(context.TODO(), bson.M{"round": s.Round}).Decode(&raw)
	notFound := err == mongo.ErrNoDocuments

	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing record failed: %w", err)
	}

	// Create bson UpcomingMatchDoc
	filter := bson.M{"round": s.Round}
	upcomingMatchDoc := UpcomingMatchDoc{
		Round:            s.Round,
		ScheduledMatches: scheduledMatches,
		TTL:              DetermineTTL(scheduledMatches),
	}
	update := bson.M{"$set": upcomingMatchDoc}

	fmt.Println("updating match schedule in db...")

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

// Function to check if scheduled matches are populated in the db. Functions like getting and setting predictions are relient on this data
// being present, so this function gives a way to check if that data actually exists before the program tries to use it
// Preconditions: Receives receiver pointer for Score which contains information about the DB
// Postconditions: Returns nil, or an error if the collection doesn't exist, the results are empty, or another error occurs
func (s *Store) EnsureScheduledMatches() error {
	var result struct {
		ScheduledMatches []external.ScheduledMatch `bson:"scheduled_matches"`
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
