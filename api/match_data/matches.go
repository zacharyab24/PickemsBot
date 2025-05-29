/* matches.go
 * Contains the interfaces, structs and helper functions used by the match package related to data fetching
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */

package match_data

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

// Helper struct to track if the team has advanced past a round of the elim bracket, been eliminated or pending
type TeamProgress struct {
	Round string
	Status string
}

// Struct for single elimination results
type EliminationResult struct {
    Progression map[string]TeamProgress 
}

func (e EliminationResult) GetType() string {
    return "single-elimination"
}

// Struct for a binary tree node
// This tree is used for the results of the finals section, or any other single elimination tournament
type MatchNode struct {
	Id string
	Team1 string
	Team2 string
	Winner string
}

type UpcomingMatch struct {
	Team1 string
	Team2 string
	EpochTime int64
	BestOf string
	StreamUrl string
}