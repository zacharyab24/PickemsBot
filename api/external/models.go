/* models.go
 * This file contains the models used by the external package when fetching data from external sources
 * Authors: Zachary Bower
 */

package external

import "pickems-bot/api/shared"

// MatchResult is an interface for match results. Used to unify the return types of swiss and single-elimination for GetMatchData
type MatchResult interface {
	GetType() string
}

// SwissResult is a struct for swiss tournament results
type SwissResult struct {
	Scores map[string]string
}

// GetType returns the tournament format type
func (s SwissResult) GetType() string {
	return "swiss"
}

// EliminationResult is a struct for single elimination tournament results
type EliminationResult struct {
	Progression map[string]shared.TeamProgress
}

// GetType returns the tournament format type
func (e EliminationResult) GetType() string {
	return "single-elimination"
}

// MatchNode represents a single match in a tournament bracket
type MatchNode struct {
	ID     string
	Team1  string
	Team2  string
	Winner string
}

// ScheduledMatch represents a scheduled match with timing and streaming information
type ScheduledMatch struct {
	Team1     string
	Team2     string
	EpochTime int64
	BestOf    string
	StreamURL string
	Finished  bool
}
