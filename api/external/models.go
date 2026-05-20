/* models.go
 * This file contains the models used by the external package when fetching data from external sources
 * Authors: Zachary Bower
 */

package external

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
