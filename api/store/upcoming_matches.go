/* upcoming_matches.go
 * Contains the methods for interacting with the upcoming_matches collection
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"errors"
	"fmt"
	"pickems-bot/api/external"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Function used to fetch upcoming matches from db
// Preconditions: Receives name of database as string (e.g. user_pickems), recieves name of collection as a string
// (e.g. upcoming_matches) and round as a string (e.g. stage_1)
// Postconditions: Returns slice of upcoming matches or error message if the operation was unsuccessful
func (s *Store) FetchUpcomingMatchesFromDb() ([]external.UpcomingMatch, error){
	opts := options.FindOne()
	
	// Get UpcomingMatchDoc result from db
	var res UpcomingMatchDoc
	err := s.Collections.UpcomingMatches.FindOne(context.TODO(), bson.D{{Key: "round", Value: s.Round}}, opts).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("no results found in db")
		}
		return nil, fmt.Errorf("error fetching results from db: %w", err)
	}

	return res.UpcomingMatches, nil
}
