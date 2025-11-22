/* store_test.go
 * Contains unit tests for store.go and store_interface.go
 * Authors: Zachary Bower
 */

package store

import (
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
