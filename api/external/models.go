/* models.go
 * This file contains the models used by the external package when fetching data from external sources
 * Authors: Zachary Bower
 */

package external

import "pickems-bot/api/shared"

// Interface for MatchResults. Used to unify the return types of swiss and single-elimination for GetMatchData
type MatchResult interface {
	GetType() string
}

// Struct for swiss results
type SwissResult struct {
	Scores map[string]string
}

func (s SwissResult) GetType() string {
	return "swiss"
}

// Struct for single elimination results
type EliminationResult struct {
	Progression map[string]shared.TeamProgress
}

func (e EliminationResult) GetType() string {
	return "single-elimination"
}

type MatchNode struct {
	Id     string
	Team1  string
	Team2  string
	Winner string
}

type ScheduledMatch struct {
	Team1     string
	Team2     string
	EpochTime int64
	BestOf    string
	StreamUrl string
	Finished bool
}