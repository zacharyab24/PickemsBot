/* models.go
 * This file contains the models used by the external package when fetching data from external sources
 * Authors: Zachary Bower
 */

package sources

// MatchNode represents a single match in a tournament bracket
type MatchNode struct {
	ID      string `bson:"id"`
	Team1   string `bson:"team1"`
	Team2   string `bson:"team2"`
	Winner  string `bson:"winner"`
	Score   string `bson:"score"`   // series score ("2-1") for BoX, map score ("13-10") for BO1; "" if unfinished
	Section string `bson:"section"` // round label from Liquipedia (e.g. "Round 1", "Upper Bracket Round 2")
	Status  string // source-specific status string (e.g. "finished", "running", "not_started" for PandaScore)
}

// ScheduledMatch represents a scheduled match with timing and streaming information
type ScheduledMatch struct {
	Team1     string
	Team2     string
	EpochTime int64
	BestOf    string
	StreamURL string
	Finished  bool
	Live      bool
}
