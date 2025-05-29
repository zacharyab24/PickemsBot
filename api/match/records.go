/* records.go
 * Contains the interfaces, structs and helper functions used by the match package related to the database
 * Authors: Zachary Bower
 * Last modified: 28/05/2025
 */

package match

type ResultRecord interface {
	GetType() string
	GetRound() string
	GetTTL() int64
	GetTeams() map[string]interface{}

}

// SwissResultRecord represents the way data will be stored in the DB for a swiss style bracket
type SwissResultRecord struct {
	Round string            `bson:"round,omitempty"`
	TTL   int64         	`bson:"ttl,omitempty"`
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
    Round string `bson:"round,omitempty"`
    TTL int64 `bson:"ttl,omitempty"`
	Progression map[string]TeamProgress `bson:"progression,omitempty"`
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
	result := make(map[string]interface{}, len(e.Progression))
	for k, v := range e.Progression {
		result[k] = v
	}
	return result
}

type UpcomingMatchDoc struct {
	Round string `bson:"round,omitempty"`
	UpcomingMatches []UpcomingMatch `bson:"upcoming_matches,omitempty"`
}