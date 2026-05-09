package format

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"pickems-bot/api/external"
	"pickems-bot/api/shared"

	"go.mongodb.org/mongo-driver/bson"
)

// SwissResult is the unified in-memory + on-disk representation of a Swiss
// tournament's match data. Teams maps team name → score string (e.g. "3-0").
type SwissResult struct {
	Round string            `bson:"round,omitempty"`
	Teams map[string]string `bson:"teams,omitempty"`
}

// GetType returns the Swiss format identifier.
func (SwissResult) GetType() Kind { return Swiss }

// GetRound returns the tournament round this result is for.
func (s SwissResult) GetRound() string { return s.Round }

// GetTeamNames returns the team-name keys from Teams. Order is not guaranteed.
func (s SwissResult) GetTeamNames() []string {
	names := make([]string, 0, len(s.Teams))
	for name := range s.Teams {
		names = append(names, name)
	}
	return names
}

// swissFormat implements Format for Swiss-system tournaments.
type swissFormat struct{}

var _ Format = swissFormat{}

func init() { register(swissFormat{}) }

func (swissFormat) Name() Kind { return Swiss }

// RequiredPredictions returns the total picks needed for a 3-win / 3-loss
// Swiss bracket of the given size. Breakdown:
//
//	3-0 picks:        T/8
//	3-1, 3-2 picks:   3T/8  (half the field advances, minus the 3-0s)
//	0-3 picks:        T/8
//	-----------------------
//	total:            5T/8
//
// Assumes proper Swiss pairing (winners vs winners, losers vs losers) and a
// team count that's a multiple of 8 — the standard 16-team CS major shape
// returns 10. Non-conforming team counts integer-divide and may not reflect
// the real bracket exactly.
func (swissFormat) RequiredPredictions(teamCount int) int { return 5 * teamCount / 8 }

func (swissFormat) GeneratePrediction(user shared.User, round string, teams []string) (shared.Prediction, error) {
	// Set generic attributes for Prediction struct
	prediction := shared.Prediction{
		UserID:   user.UserID,
		Username: user.Username,
		Format:   "swiss",
		Round:    round,
	}

	win, advance, lose := setSwissPredictions(teams)
	prediction.Win = win
	prediction.Advance = advance
	prediction.Lose = lose

	return prediction, nil
}

func (swissFormat) CalculateScore(p shared.Prediction, r MatchResult) (shared.ScoreResult, string, error) {
	result, ok := r.(SwissResult)
	if !ok {
		return shared.ScoreResult{}, "", fmt.Errorf("swiss: expected SwissResult, got %T", r)
	}
	scores := result.Teams

	var succeeded, pending, failed int
	var response strings.Builder

	// [3-0]
	response.WriteString("[3-0]\n")
	bucket, err := evaluateBucket(p.Win, scores, func(wins, loses int) bucketStatus {
		if loses >= 1 {
			return statusFailed
		} else if wins != 3 {
			return statusPending
		}
		return statusSucceeded
	}, &response)
	if err != nil {
		return shared.ScoreResult{}, "", err
	}
	succeeded += bucket.Successes
	pending += bucket.Pending
	failed += bucket.Failed

	// [3-1, 3-2]
	response.WriteString("[3-1, 3-2]\n")
	bucket, err = evaluateBucket(p.Advance, scores, func(wins, loses int) bucketStatus {
		if loses == 3 || (wins == 3 && loses == 0) {
			return statusFailed
		} else if wins < 3 {
			return statusPending
		}
		return statusSucceeded
	}, &response)
	if err != nil {
		return shared.ScoreResult{}, "", err
	}
	succeeded += bucket.Successes
	pending += bucket.Pending
	failed += bucket.Failed

	// [0-3]
	response.WriteString("[0-3]\n")
	bucket, err = evaluateBucket(p.Lose, scores, func(wins, loses int) bucketStatus {
		if wins >= 1 {
			return statusFailed
		} else if loses != 3 {
			return statusPending
		}
		return statusSucceeded
	}, &response)
	if err != nil {
		return shared.ScoreResult{}, "", err
	}
	succeeded += bucket.Successes
	pending += bucket.Pending
	failed += bucket.Failed

	return shared.ScoreResult{
		Successes: succeeded,
		Pending:   pending,
		Failed:    failed,
	}, response.String(), nil
}

// ExtractMatchListID parses wikitext and extracts the `Matchlist` id
func (swissFormat) ExtractMatchListIDs(wikitext string) ([]string, Kind, error) {
	var re *regexp.Regexp
	re = regexp.MustCompile(`(?s)\{\{\s*Matchlist\s*\|([^}]*)\}\}`) // {{Matchlist ...}} templates used in swiss tournaments
	return extractMatchListIds(wikitext, re)

}

// DecodeBSON unmarshals a Swiss BSON record back into a SwissResult.
func (swissFormat) DecodeBSON(b []byte) (MatchResult, error) {
	var s SwissResult
	if err := bson.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("swiss: failed to decode BSON: %w", err)
	}
	return s, nil
}

// BuildFromMatchNodes assembles a SwissResult from parsed match nodes.
// Phase 3 will inline external.CalculateSwissScores into this package.
func (swissFormat) BuildFromMatchNodes(nodes []external.MatchNode, round string) (MatchResult, error) {
	scores, err := calculateSwissScores(nodes)
	if err != nil {
		return nil, fmt.Errorf("swiss: error calculating scores: %w", err)
	}
	return SwissResult{Round: round, Teams: scores}, nil
}

// bucketStatus is the per-team verdict produced by an evaluateBucket
// classifier — typed instead of a string so typos are caught at compile time
// and the display string is one place (the String method).
type bucketStatus int

const (
	statusSucceeded bucketStatus = iota
	statusPending
	statusFailed
)

func (s bucketStatus) String() string {
	switch s {
	case statusSucceeded:
		return "[Succeeded]"
	case statusPending:
		return "[Pending]"
	case statusFailed:
		return "[Failed]"
	default:
		return "[Unknown]"
	}
}

// evaluateBucket scores a single Swiss prediction bucket (e.g. "3-0 picks")
// against the live score map. classify maps a team's current wins/losses to a
// bucketStatus; we tally and write a per-team line into builder as we go.
func evaluateBucket(teams []string, scores map[string]string, classify func(wins, loses int) bucketStatus, builder *strings.Builder) (shared.ScoreResult, error) {
	var succeeded, pending, failed int

	for _, team := range teams {
		score, ok := scores[team]
		if !ok {
			builder.WriteString(fmt.Sprintf("%s: [Missing score] %s\n", team, statusFailed))
			failed++
			continue
		}

		if len(score) != 3 || score[1] != '-' {
			return shared.ScoreResult{}, fmt.Errorf("invalid score format: %s", score)
		}

		wins, err := strconv.Atoi(string(score[0]))
		if err != nil {
			return shared.ScoreResult{}, err
		}
		loses, err := strconv.Atoi(string(score[2]))
		if err != nil {
			return shared.ScoreResult{}, err
		}

		status := classify(wins, loses)
		builder.WriteString(fmt.Sprintf("%s: %s %s\n", team, score, status))

		switch status {
		case statusSucceeded:
			succeeded++
		case statusPending:
			pending++
		case statusFailed:
			failed++
		}
	}
	return shared.ScoreResult{Successes: succeeded, Pending: pending, Failed: failed}, nil
}

// CalculateSwissScores process match nodes and calculate swiss score. Called by BuildMatchNodes
func calculateSwissScores(matchNodes []external.MatchNode) (map[string]string, error) {
	var teams []string
	wins := make(map[string]int)
	loses := make(map[string]int)

	for i := range matchNodes {
		node := matchNodes[i]

		// Check if teams are in teams slice
		if !slices.Contains(teams, node.Team1) {
			teams = append(teams, node.Team1)
		}
		if !slices.Contains(teams, node.Team2) {
			teams = append(teams, node.Team2)
		}

		// Update win and loss maps
		if node.Winner == "TBD" {
			continue
		}
		if node.Winner == node.Team1 {
			wins[node.Team1]++
			loses[node.Team2]++
		} else if node.Winner == node.Team2 {
			wins[node.Team2]++
			loses[node.Team1]++
		} else {
			// Unexpected winner value skip
			continue
		}

	}

	scores := make(map[string]string)
	for _, team := range teams {
		//Skip any placeholder teams names
		if team == "TBD" {
			continue
		}
		scores[team] = fmt.Sprintf("%d-%d", wins[team], loses[team])
	}

	return scores, nil
}

// setSwissPredictions is a helper to bind the input string slice to the 3 output slices: win, advance and lose
func setSwissPredictions(teams []string) ([]string, []string, []string) {
	numTeams := len(teams)
	numWin := numTeams / 8
	numAdvance := numTeams / 2
	numLose := 5 * numTeams / 8

	win := teams[0:numWin]
	advance := teams[numWin:numAdvance]
	lose := teams[numAdvance:numLose]

	return win, advance, lose
}
