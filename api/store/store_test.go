/* store_test.go
 * Contains unit tests for store.go and store_interface.go
 * Authors: Zachary Bower
 */

package store

import (
	"context"
	"os"
	"testing"
)

// Test getter methods
func TestStore_GetRound(t *testing.T) {
	s := &Store{Round: "test_round"}
	if s.GetRound() != "test_round" {
		t.Errorf("Expected 'test_round', got '%s'", s.GetRound())
	}
}

func TestStore_GetPage(t *testing.T) {
	s := &Store{Page: "Test/Page"}
	if s.GetPage() != "Test/Page" {
		t.Errorf("Expected 'Test/Page', got '%s'", s.GetPage())
	}
}

func TestStore_GetOptionalParams(t *testing.T) {
	s := &Store{OptionalParams: "?param=value"}
	if s.GetOptionalParams() != "?param=value" {
		t.Errorf("Expected '?param=value', got '%s'", s.GetOptionalParams())
	}
}

func TestStore_GetDatabase(t *testing.T) {
	// Test that the getter works - actual database would be set by NewStore
	s := &Store{}
	result := s.GetDatabase()

	// Just verify method exists and compiles correctly
	_ = result
}

func TestStore_GetClient(t *testing.T) {
	s := &Store{Client: nil}
	result := s.GetClient()

	// Just test that method exists and returns (even if nil)
	_ = result
}
// Integration test for NewStore
func TestNewStore_Integration(t *testing.T) {
	mongoURI := os.Getenv("MONGO_TEST_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://192.168.1.105:27017/?directConnection=true&serverSelectionTimeoutMS=2000"
	}

	store, err := NewStore("test_db", mongoURI, "Test/Page", "?param=value", "test-round")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Client.Disconnect(context.TODO())

	// Verify fields are set correctly
	if store.GetRound() != "test-round" {
		t.Errorf("Expected round 'test-round', got '%s'", store.GetRound())
	}
	if store.GetPage() != "Test/Page" {
		t.Errorf("Expected page 'Test/Page', got '%s'", store.GetPage())
	}
	if store.GetOptionalParams() != "?param=value" {
		t.Errorf("Expected params '?param=value', got '%s'", store.GetOptionalParams())
	}

	// Verify database connection
	db := store.GetDatabase()
	if db == nil {
		t.Error("Expected database to be set, got nil")
	}
	if db.Name() != "test_db" {
		t.Errorf("Expected database name 'test_db', got '%s'", db.Name())
	}

	// Verify client connection
	client := store.GetClient()
	if client == nil {
		t.Error("Expected client to be set, got nil")
	}

	// Verify collections are initialized
	if store.Collections.Predictions == nil {
		t.Error("Expected Predictions collection to be initialized")
	}
	if store.Collections.MatchResults == nil {
		t.Error("Expected MatchResults collection to be initialized")
	}
	if store.Collections.MatchSchedule == nil {
		t.Error("Expected MatchSchedule collection to be initialized")
	}
}
