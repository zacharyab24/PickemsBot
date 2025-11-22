/* models_test.go
 * Contains unit tests for models.go
 * Authors: Zachary Bower
 */

package external

import (
	"pickems-bot/api/shared"
	"testing"
)

// Test SwissResult GetType
func TestSwissResult_GetType(t *testing.T) {
	sr := SwissResult{Scores: make(map[string]string)}
	if sr.GetType() != "swiss" {
		t.Errorf("Expected 'swiss', got '%s'", sr.GetType())
	}
}

// Test EliminationResult GetType
func TestEliminationResult_GetType(t *testing.T) {
	er := EliminationResult{Progression: map[string]shared.TeamProgress{}}
	if er.GetType() != "single-elimination" {
		t.Errorf("Expected 'single-elimination', got '%s'", er.GetType())
	}
}
