/* repository.go
 * Contains the logic for interacting with the database (MongoDB)
 * Authors: Zachary Bower
 * Last modified: 28/05/2025
 */

package match

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var Client *mongo.Client
var upcomingMatches []UpcomingMatch // Keep an in memory copy of upcoming matches to reduce DB lookups

// Function used to store match results in the db
// Preconditions: Receives name of database as a string (e.g. user_pickems), receives name of collection as a string 
// (e.g. PW Shanghai Major 2024_elimination_predictions, and MatchResult inferface containing the data to be stored
// Postconditions: Returns boolean for if the operation was successful, and an error message if it was not
func StoreMatchResults(databaseName string, collectionName string, matchResult MatchResult, round string) (bool, error) {
	coll := Client.Database(databaseName).Collection(collectionName)
	
	// Get raw document from db
	var raw bson.M
	err := coll.FindOne(context.TODO(), bson.M{"round": round}).Decode(&raw)
	
	if err != nil {
		// No existing record was found, we need to insert a new one
		if err == mongo.ErrNoDocuments {
			
			// TODO: implement this
			return true, nil
		}
		// Anything else is an error looking up
		return false, fmt.Errorf("lookup for existing record failed: %w", err)
	}
	
	switch matchResult.GetType() {
	case "swiss":
		swiss, ok := matchResult.(SwissResult)
		if !ok {
			return false, fmt.Errorf("could not convert MatchResult to SwissResult")
		}
		var existing SwissResultRecord
		bsonBytes, _ := bson.Marshal(raw)
		if err := bson.Unmarshal(bsonBytes, &existing); err != nil {
			return false, fmt.Errorf("failed to parse existing Swiss record: %w", err)
		}
		return updateSwissResult(coll, existing, swiss.Scores)

	case "single-elimination":
		elimination, ok := matchResult.(EliminationResult)
		if !ok {
			return false, fmt.Errorf("could not convert MatchResult to EliminationResult")
		}
		var existing EliminationResultRecord
		bsonBytes, _ := bson.Marshal(raw)
		if err := bson.Unmarshal(bsonBytes, &existing); err != nil {
			return false, fmt.Errorf("failed to parse existing Elimination record: %w", err)
		}
		return updateEliminationResult(coll, existing, elimination.Progression)

	default:
		return false, fmt.Errorf("unknown match result type: %s", matchResult.GetType())
	}
}

// Function to update a swiss record in the database
// Preconditions: Receives pointer to mongodb colletion, existing SwissResultsRecord in db, and scores map
// Postconditons: Updates the DB with new scores map, new TTL and returns true, or if something goes wrong returns false and error
func updateSwissResult(coll *mongo.Collection, existing SwissResultRecord, scores map[string]string) (bool, error) {
	// Updating existing record
	existing.TTL = DetermineTTL(upcomingMatches)
	existing.Teams = scores

	// Define update fields
	update := bson.M {
		"$set": bson.M{
			"TTL": existing.TTL,
			"teams": existing.Teams,
		},
	}

	filter := bson.M{"Round": existing.Round}
	res, err := coll.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return false, fmt.Errorf("failed to update document: %w", err)
	}

	return res.ModifiedCount > 0, nil // True if there was a record modified
}

// Function to update a swiss record in the database
// Preconditions: Receives pointer to mongodb colletion, existing EliminationResultRecord in db, and progression map
// Postconditons: Updates the DB with new progression map, new TTL and returns true, or if something goes wrong returns false and error
func updateEliminationResult(coll *mongo.Collection, existing EliminationResultRecord, progression map[string]TeamProgress) (bool, error) {
	// Updating existing record
	existing.TTL = DetermineTTL(upcomingMatches)
	existing.Progression = progression

	// Define update fields
	update := bson.M {
		"$set": bson.M{
			"TTL": existing.TTL,
			"teams": existing.Progression,
		},
	}

	filter := bson.M{"Round": existing.Round}
	res, err := coll.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return false, fmt.Errorf("failed to update document: %w", err)
	}

	return res.ModifiedCount > 0, nil // True if there was a record modified
}


// Function for calculating TTL for result caching. If there are ongoing matches this is shortTTL, else normalTTL. These
// values are defined in a const within the function
// Preconditions: Receives slice of UpcomingMatch which contains information about the match
// Postconditions: Returns time.Duration with the value for the TTL
func DetermineTTL(matches []UpcomingMatch) time.Time {
	now := time.Now().Unix()

	const (
		shortTTL = 3 * time.Minute // When there is a match ongoing TTL is 3 minutes
		normalTTL = 30 * time.Minute // Else it is 30
	)

	for _, match := range matches {
		var estimatedMatchDuration int64
		switch strings.ToLower(match.BestOf) {
		case "1": //BO1 time estimate is 90 mins
			estimatedMatchDuration = 1 * 60 * 90
		case "3": //BO3 time estimate is 4 hours
			estimatedMatchDuration = 4 * 60 * 60
		case "5": // BO5 time estimate is 6 hours
			estimatedMatchDuration = 6 * 60 * 60
		default: // If there is different or missing BestOf value default to 3 hours
			estimatedMatchDuration = 3 * 60 * 60
		}

		startTime := match.EpochTime
		finishedTime := startTime + estimatedMatchDuration

		// We are defining an ongoing match to be a match between the start time and estimated finish time
		// It would be possible to check if the match has finished from the Liquipedia API but that adds an extra request
		// to the rate limiter
		if now >= startTime && now <= finishedTime {
			return time.Now().Add(shortTTL)
		}
	}
	// No ongoing match
	return time.Now().Add(normalTTL)
}