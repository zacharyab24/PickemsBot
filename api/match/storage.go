/* repository.go
 * Contains the logic for interacting with the database (MongoDB)
 * Authors: Zachary Bower
 * Last modified: 28/05/2025
 */

package match

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Client *mongo.Client
//var upcomingMatches []UpcomingMatch // Keep an in memory copy of upcoming matches to reduce DB lookups

// Function to init DB connection from main
func Init(uri string) error {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	if err := client.Ping(context.TODO(), nil); err != nil {
		return err
	}
	Client = client
	return nil
}


// Function used to store match results in the db
// Preconditions: Receives name of database as a string (e.g. user_pickems), receives name of collection as a string,
// (e.g. PW Shanghai Major 2024_elimination_predictions, and MatchResult inferface containing the data to be stored
// Postconditions: Returns boolean for if the operation was successful, and an error message if it was not
func StoreMatchResults(dbName string, collectionName string, matchResult MatchResult, round string, upcomingMatches []UpcomingMatch) error {
	coll := Client.Database(dbName).Collection(collectionName)

	// Attempt to find an existing document
	var raw bson.M
	err := coll.FindOne(context.TODO(), bson.M{"round": round}).Decode(&raw)
	notFound := err == mongo.ErrNoDocuments

	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing record failed: %w", err)
	}

	ttl := DetermineTTL(upcomingMatches)

	var update bson.M
	var newRecord interface{}
	var filter = bson.M{"round": round}

	switch typed := matchResult.(type) {
	case SwissResult:
		update = bson.M{
			"$set": bson.M{
				"ttl":   ttl,
				"teams": typed.Scores,
			},
		}
		newRecord = bson.M{
			"type": typed.GetType(),
			"round": round,
			"ttl": ttl,
			"teams": typed.Scores,
		}

	case EliminationResult:
		update = bson.M{
			"$set": bson.M{
				"ttl":   ttl,
				"teams": typed.Progression,
			},
		}
		newRecord = bson.M{
			"type": typed.GetType(),
			"round": round,
			"ttl": ttl,
			"teams": typed.Progression,
		}

	default:
		return fmt.Errorf("unknown match result type: %s", matchResult.GetType())
	}

	// Perform insert or update
	if notFound {
		_, err := coll.InsertOne(context.TODO(), newRecord)
		if err != nil {
			return fmt.Errorf("failed to insert new match result: %w", err)
		}
		return nil
	}

	_, err = coll.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update match result: %w", err)
	}
	return nil
}

// Function to retrieve match results from the DB
// Preconditions:
// Postconditions:
func FetchMatchResultsFromDb(dbName string, collectionName string, round string) (MatchResult, error) {
	coll := Client.Database(dbName).Collection(collectionName)
	opts := options.FindOne()
	
	// MatchResult is an interface, which can't be decoded by MongoDB's driver. Instead need to get raw and convert to interface later
	var raw bson.M

	err := coll.FindOne(context.TODO(), bson.D{{Key: "round", Value: round}}, opts).Decode(&raw)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("no results found in db")
		}
		return nil, fmt.Errorf("error fetching results from db: %w", err)
	}

	// Determine which type of MatchResult we fetched
	resultType, ok := raw["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid `type` field in document")
	}

	bsonBytes, err := bson.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw bson: %w", err)
	}

	// Decode results and map to correct struct for format
	switch resultType {
	case "swiss":
		var swiss SwissResultRecord
		if err := bson.Unmarshal(bsonBytes, &swiss); err != nil {
			return nil, fmt.Errorf("failed to decode SwissResultRecord: %w", err)
		}
		return swiss, nil
	case "single-elimination":
		var elim EliminationResultRecord
		if err := bson.Unmarshal(bsonBytes, &elim); err != nil {
			return nil, fmt.Errorf("failed to decode EliminationResultRecord: %w", err)
		}
		return elim, nil
	default:
		return nil, fmt.Errorf("unknown match result type: %s", resultType)
	}
}

// Function for calculating TTL for result caching. If there are ongoing matches this is shortTTL, else normalTTL. These
// values are defined in a const within the function
// Preconditions: Receives slice of UpcomingMatch which contains information about the match
// Postconditions: Returns time.Duration with the value for the TTL
func DetermineTTL(matches []UpcomingMatch) int64 {
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
			return time.Now().Add(shortTTL).Unix()
		}
	}
	// No ongoing match
	return time.Now().Add(normalTTL).Unix()
}