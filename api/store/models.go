/* models.go
 * This file contain the interfaces, structs and helper functions that relate to DB objects
 * Authors: Zachary Bower
 */

package store

import (
	"fmt"
	"pickems-bot/api/external"
	"pickems-bot/api/shared"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Prediction struct {
	// Generic attributes
	Id       primitive.ObjectID `bson:"_id,omitempty"`
	UserId   string             `bson:"userid,omitempty"`
	Username string             `bson:"username,omitempty"`
	Format   string             `bson:"format,omitempty"` // "swiss" or "single-elimination"
	Round    string             `bson:"round,omitempty"`

	// Swiss-specific attributes
	Win     []string `bson:"win,omitempty"`
	Advance []string `bson:"advance,omitempty"`
	Lose    []string `bson:"lose,omitempty"`

	// Elimination specific attributes
	Progression map[string]shared.TeamProgress `bson:"progression,omitempty"`
}

type ResultRecord interface {
	GetType() string
	GetRound() string
	GetTTL() int64
	GetTeams() map[string]interface{}
}

// SwissResultRecord represents the way data will be stored in the DB for a swiss style bracket
type SwissResultRecord struct {
	Round string            `bson:"round,omitempty"`
	TTL   int64             `bson:"ttl,omitempty"`
	Teams map[string]string `bson:"teams,omitempty"`
}

func (s SwissResultRecord) GetType() string {
	return "swiss"
}

func (s SwissResultRecord) GetRound() string {
	return s.Round
}

func (s SwissResultRecord) GetTTL() int64 {
	return s.TTL
}

func (s SwissResultRecord) GetTeams() map[string]interface{} {
	result := make(map[string]interface{}, len(s.Teams))
	for k, v := range s.Teams {
		result[k] = v
	}
	return result
}

// EliminationRecordResult represents the way data will be stored in the DB for a single-elimination bracket
type EliminationResultRecord struct {
	Round string                  `bson:"round,omitempty"`
	TTL   int64                   `bson:"ttl,omitempty"`
	Teams map[string]shared.TeamProgress `bson:"teams,omitempty"`
}

func (e EliminationResultRecord) GetType() string {
	return "single-elimination"
}

func (e EliminationResultRecord) GetRound() string {
	return e.Round
}

func (e EliminationResultRecord) GetTTL() int64 {
	return e.TTL
}

func (e EliminationResultRecord) GetTeams() map[string]interface{} {
	result := make(map[string]interface{}, len(e.Teams))
	for k, v := range e.Teams {
		// Convert TeamProgress struct to map[string]interface{}
		teamData := map[string]interface{}{
			"round":  v.Round,
			"status": v.Status,
		}
		result[k] = teamData
	}
	return result
}

// Upcoming Match Document struct
type UpcomingMatchDoc struct {
	Round string `bson:"round,omitempty"`
	UpcomingMatches []external.UpcomingMatch `bson:"upcoming_matches,omitempty"`
}

// Function to convert RecordResult interface to MatchResult interface. Used when getting data from the db
// Preconditions: none
// Postconditions: Returns a MatchResult or error if it occurs 
func ToMatchResult(r ResultRecord) (external.MatchResult, error) {
    switch r.GetType() {
    case "swiss":
        scores := make(map[string]string)
        for team, val := range r.GetTeams() {
            strVal, ok := val.(string)
            if !ok {
                return nil, fmt.Errorf("invalid score for team %s", team)
            }
            scores[team] = strVal
        }
        return external.SwissResult{Scores: scores}, nil
    case "single-elimination":
        progression := make(map[string]shared.TeamProgress)
        for team, val := range r.GetTeams() {
            raw, ok := val.(map[string]interface{})
            if !ok {
                return nil, fmt.Errorf("invalid progression format for team %s", team)
            }
            
            // Initialize TeamProgress struct
            tp := shared.TeamProgress{}
            
            if round, ok := raw["round"].(string); ok {
                tp.Round = round
            }
            if status, ok := raw["status"].(string); ok {
                tp.Status = status
            }
            progression[team] = tp
        }
        return external.EliminationResult{Progression: progression}, nil
    default:
        return nil, fmt.Errorf("unknown result type: %s", r.GetType())
    }
}

type ScoreResult struct {
	Successes int
	Pending int
	Failed int
}