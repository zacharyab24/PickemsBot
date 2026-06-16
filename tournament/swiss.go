package tournament

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"pickems-bot/models"
	"pickems-bot/sources"

	"go.mongodb.org/mongo-driver/bson"
)

// SwissResult is the unified in-memory + on-disk representation of a Swiss
// tournament's match data. Teams maps team name → score string (e.g. "3-0").
type SwissResult struct {
	Round string            `bson:"round,omitempty"`
	Teams map[string]string `bson:"teams,omitempty"`
}

// SwissReport is the structured result returned by swissFormat.CalculateScore.
// Each bucket slice holds one entry per predicted team; the handler uses these
// to build a Discord embed instead of parsing a raw string.
type SwissReport struct {
	WinPicks     []BucketEntry // 3-0 predictions
	AdvancePicks []BucketEntry // 3-1 / 3-2 predictions
	LosePicks    []BucketEntry // 0-3 predictions
	Score        models.ScoreResult
}

// FormatKind implements ScoreReport.
func (SwissReport) FormatKind() Kind { return Swiss }

// GetScore implements ScoreReport.
func (s SwissReport) GetScore() models.ScoreResult { return s.Score }

// BucketEntry is the per-team result for one Swiss prediction bucket.
type BucketEntry struct {
	Team   string
	Score  string // current record e.g. "2-1"; empty string when missing from results
	Status BucketStatus
}

// BucketStatus is the per-team verdict produced by evaluateBucket.
// Typed instead of a string so typos are caught at compile time and the
// display string lives in one place (the String method).
type BucketStatus int

// BucketStatus values indicate whether a prediction was correct, still in progress, or wrong.
const (
	StatusSucceeded BucketStatus = iota
	StatusPending
	StatusFailed
)

func (s BucketStatus) String() string {
	switch s {
	case StatusSucceeded:
		return "✅"
	case StatusPending:
		return "⏳"
	case StatusFailed:
		return "❌"
	default:
		return "❓"
	}
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

func (swissFormat) PredictionFields(p models.Prediction) ([]models.PredictionField, error) {
	if len(p.Progression) > 0 {
		return nil, fmt.Errorf("swiss prediction contains unexpected progression data")
	}
	return []models.PredictionField{
		{Name: "3-0", Value: strings.Join(p.Win, ", ")},
		{Name: "Advance", Value: strings.Join(p.Advance, ", ")},
		{Name: "0-3", Value: strings.Join(p.Lose, ", ")},
	}, nil
}

func (swissFormat) GeneratePrediction(user models.User, round string, teams []string) (models.Prediction, error) {
	// Set generic attributes for Prediction struct
	prediction := models.Prediction{
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

func (swissFormat) CalculateScore(p models.Prediction, r MatchResult) (ScoreReport, error) {
	result, ok := r.(SwissResult)
	if !ok {
		return nil, fmt.Errorf("swiss: expected SwissResult, got %T", r)
	}
	scores := result.Teams

	// [3-0]
	winEntries, err := evaluateBucket(p.Win, scores, func(wins, loses int) BucketStatus {
		if loses >= 1 {
			return StatusFailed
		} else if wins != 3 {
			return StatusPending
		}
		return StatusSucceeded
	})
	if err != nil {
		return nil, err
	}

	// [3-1, 3-2]
	advanceEntries, err := evaluateBucket(p.Advance, scores, func(wins, loses int) BucketStatus {
		if loses == 3 || (wins == 3 && loses == 0) {
			return StatusFailed
		} else if wins < 3 {
			return StatusPending
		}
		return StatusSucceeded
	})
	if err != nil {
		return nil, err
	}

	// [0-3]
	loseEntries, err := evaluateBucket(p.Lose, scores, func(wins, loses int) BucketStatus {
		if wins >= 1 {
			return StatusFailed
		} else if loses != 3 {
			return StatusPending
		}
		return StatusSucceeded
	})
	if err != nil {
		return nil, err
	}

	var succeeded, pending, failed int
	for _, e := range append(append(winEntries, advanceEntries...), loseEntries...) {
		switch e.Status {
		case StatusSucceeded:
			succeeded++
		case StatusPending:
			pending++
		case StatusFailed:
			failed++
		}
	}

	return SwissReport{
		WinPicks:     winEntries,
		AdvancePicks: advanceEntries,
		LosePicks:    loseEntries,
		Score: models.ScoreResult{
			Successes: succeeded,
			Pending:   pending,
			Failed:    failed,
		},
	}, nil
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
func (swissFormat) BuildFromMatchNodes(nodes []sources.MatchNode, round string) (MatchResult, error) {
	scores, err := calculateSwissScores(nodes)
	if err != nil {
		return nil, fmt.Errorf("swiss: error calculating scores: %w", err)
	}
	return SwissResult{Round: round, Teams: scores}, nil
}

// evaluateBucket scores a single Swiss prediction bucket (e.g. "3-0 picks")
// against the live score map. classify maps a team's current wins/losses to a
// BucketStatus. Returns one BucketEntry per team; missing teams get an empty
// Score and StatusFailed.
func evaluateBucket(teams []string, scores map[string]string, classify func(wins, loses int) BucketStatus) ([]BucketEntry, error) {
	entries := make([]BucketEntry, 0, len(teams))

	for _, team := range teams {
		score, ok := scores[team]
		if !ok {
			entries = append(entries, BucketEntry{Team: team, Score: "", Status: StatusFailed})
			continue
		}

		if len(score) != 3 || score[1] != '-' {
			return nil, fmt.Errorf("invalid score format: %s", score)
		}

		wins, err := strconv.Atoi(string(score[0]))
		if err != nil {
			return nil, err
		}
		loses, err := strconv.Atoi(string(score[2]))
		if err != nil {
			return nil, err
		}

		entries = append(entries, BucketEntry{
			Team:   team,
			Score:  score,
			Status: classify(wins, loses),
		})
	}
	return entries, nil
}

// calculateSwissScores processes match nodes and calculates Swiss scores. Called by BuildFromMatchNodes.
func calculateSwissScores(matchNodes []sources.MatchNode) (map[string]string, error) {
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
			// Unexpected winner value — skip
			continue
		}

	}

	scores := make(map[string]string)
	for _, team := range teams {
		// Skip any placeholder team names
		if team == "TBD" {
			continue
		}
		scores[team] = fmt.Sprintf("%d-%d", wins[team], loses[team])
	}

	return scores, nil
}

// NormalizeSwissSections rewrites the Section field on each node to a canonical
// "Round N" label so the DB representation is identical regardless of data source.
//
// Supported input formats:
//   - "Round N"                  → "Round N"  (already canonical)
//   - "Round N: Team vs Team"    → "Round N"  (PandaScore per-match labels)
//   - "W:L" record format        → "Round N"  where N = W+L+1  (Liquipedia)
func NormalizeSwissSections(nodes []sources.MatchNode) []sources.MatchNode {
	out := make([]sources.MatchNode, len(nodes))
	for i, n := range nodes {
		n.Section = canonicalSwissSection(n.Section)
		out[i] = n
	}
	return out
}

func canonicalSwissSection(section string) string {
	// "Round N" or "Round N: ..." — keep just "Round N"
	if strings.HasPrefix(section, "Round ") {
		after := strings.TrimPrefix(section, "Round ")
		numStr := after
		if idx := strings.IndexByte(after, ':'); idx >= 0 {
			numStr = strings.TrimSpace(after[:idx])
		}
		if n, err := strconv.Atoi(numStr); err == nil && n > 0 {
			return fmt.Sprintf("Round %d", n)
		}
	}
	// "W:L" record format: N = W + L + 1
	parts := strings.SplitN(section, ":", 2)
	if len(parts) == 2 {
		w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		l, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 == nil && err2 == nil {
			return fmt.Sprintf("Round %d", w+l+1)
		}
	}
	return section
}

// setSwissPredictions splits a flat prediction list into the three Swiss
// buckets: 3-0 (win), 3-1/3-2 (advance), 0-3 (lose).
//
// The input slice has exactly 5N/8 entries (where N is the total team count),
// so each bucket's share out of the input is:
//
//	3-0:     1/5 of input  (= N/8)
//	advance: 3/5 of input  (= 3N/8)
//	0-3:     1/5 of input  (= N/8)
//
// Dividing by the input length directly — as the old code did — produced wrong
// bucket sizes because it treated the 5N/8-length list as if it were N.
func setSwissPredictions(teams []string) ([]string, []string, []string) {
	T := len(teams)
	numWin := T / 5
	numLoseStart := 4 * T / 5

	win := teams[0:numWin]
	advance := teams[numWin:numLoseStart]
	lose := teams[numLoseStart:]

	return win, advance, lose
}
