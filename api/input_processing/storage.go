/* storage.go
 * Contains the logic for storing user predictions and leaderboards
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */

package input_processing

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Client *mongo.Client

// Function to init DB connection from main
// Preconditions: Receives string containing mongodb uri
// Postconditions: Establishes connection to db located at uri and updates the global value Client to be the DB connection
// or returns an error if it occurs
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

// Function to store user predictions in the db
// Preconditions: receives strings containing db name, collection name and userId, and Prediction containing the users predictions
// Postconditions: stores or updates the user's prediction stored in the db, or returns an error if the opperations was unsuccessful
func StoreUserPrediction(dbName string, collectionName string, userId string, userPrediction Prediction) error {
	coll := Client.Database(dbName).Collection(collectionName)

	// Attempt to find an existing document
	var result Prediction
	err := coll.FindOne(context.TODO(), bson.M{"userid": userId, "round": userPrediction.Round}).Decode(&result)
	notFound := err == mongo.ErrNoDocuments

	if err != nil && !notFound {
		return fmt.Errorf("lookup for existing prediction failed: %w", err)
	}

	update := bson.M {
		"$set": userPrediction,
	}
	filter := bson.M{
		"userid": userId,
		"round": userPrediction.Round,
	}

	// The user currently does not have predictions stored so we create a new document
	if notFound {
		_, err := coll.InsertOne(context.TODO(), userPrediction)
		if err != nil {
			return fmt.Errorf("failed to insert new user prediction: %w", err)
		}
		return nil
	}

	// Else update the user's existing prediction
	_, err = coll.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update existing user prediction: %w", err)
	}
	return nil
}

// Function to do DB lookup and get prediction for a user
// Preconditions: receives strings containing db name, collection name and userId
// Postconditions: returns a user's prediction if it exists, or an error if it occurs
func GetUserPrediction(dbName string, collectionName string, userId string) (Prediction, error) {
	coll := Client.Database(dbName).Collection(collectionName)
	opts := options.FindOne()

	var result Prediction

	err := coll.FindOne(context.TODO(), bson.D{{Key: "userid", Value: userId}}, opts).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Prediction{}, err
		}
		return Prediction{}, fmt.Errorf("error fetching results from db: %w", err)
	}

	return result, nil
}

// Function to do DB lookup and the predictions for all users with predictions stored for a round. Used in leaderboard calculations
// Preconditions: Receives strings containing database name, colletionname and round
// Postconditions: Returns slice of Predictions or an error if it occurs
func GetAllUserPredictions(dbName string, collectionName string, round string) ([]Prediction, error) {
	coll := Client.Database(dbName).Collection(collectionName)

	// Filter query to match documents where the round is the round sting input to the function
	filter := bson.D{{Key: "round", Value: round}}
	
	// Retrieves documents that match the filter
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		return nil, fmt.Errorf("error fetching results from db: %w", err)
	}
	
	// Unpack the cursor into a slice
	var results []Prediction
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, fmt.Errorf("error unpacking cursor into slice of predictions: %w", err)
	}
	
	return results, nil
}