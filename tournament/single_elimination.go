package tournament

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"

	"pickems-bot/models"
	"pickems-bot/sources"

	"go.mongodb.org/mongo-driver/bson"
)

// ElimPredictionEntry is the per-team result for a single-elimination prediction.
type ElimPredictionEntry struct {
	Team   string
	Round  string
	ToWin  bool // true = predicted to win the Grand Final; false = predicted to be eliminated here
	Status BucketStatus
}

// SingleElimReport is the structured result returned by singleElimFormat.CalculateScore.
type SingleElimReport struct {
	Predictions []ElimPredictionEntry
	Score       models.ScoreResult
}

// FormatKind implements ScoreReport.
func (SingleElimReport) FormatKind() Kind { return SingleElim }

// GetScore implements ScoreReport.
func (s SingleElimReport) GetScore() models.ScoreResult { return s.Score }

// EliminationResult is the unified in-memory + on-disk representation of a
// single-elimination bracket's progression data. Teams maps team name →
// TeamProgress (round reached + advanced/eliminated/pending status).
type EliminationResult struct {
	Round string                         `bson:"round,omitempty"`
	Teams map[string]models.TeamProgress `bson:"teams,omitempty"`
}

// GetType returns the single-elimination format identifier.
func (EliminationResult) GetType() Kind { return SingleElim }

// GetRound returns the tournament round this result is for.
func (e EliminationResult) GetRound() string { return e.Round }

// GetTeamNames returns the team-name keys from Teams. Order is not guaranteed.
func (e EliminationResult) GetTeamNames() []string {
	names := make([]string, 0, len(e.Teams))
	for name := range e.Teams {
		names = append(names, name)
	}
	return names
}

// singleElimFormat implements Format for single-elimination bracket tournaments.
type singleElimFormat struct{}

var _ Format = singleElimFormat{}

func init() { register(singleElimFormat{}) }

func (singleElimFormat) Name() Kind { return SingleElim }

// RequiredPredictions returns teamCount / 2 — one pick per first-round
// matchup, predicting which team advances.
func (singleElimFormat) RequiredPredictions(teamCount int) int { return teamCount / 2 }

func (singleElimFormat) GeneratePrediction(user models.User, round string, teams []string) (models.Prediction, error) {
	// Set generic attributes for Prediction struct
	prediction := models.Prediction{
		UserID:   user.UserID,
		Username: user.Username,
		Format:   "single-elimination",
		Round:    round,
	}

	progression := setEliminationPredictions(teams)
	prediction.Progression = progression

	return prediction, nil
}

func (singleElimFormat) CalculateScore(p models.Prediction, r MatchResult) (ScoreReport, error) {
	result, ok := r.(EliminationResult)
	if !ok {
		return nil, fmt.Errorf("single-elimination: expected EliminationResult, got %T", r)
	}
	results := result.Teams

	if len(p.Progression) == 0 || len(results) == 0 {
		return nil, fmt.Errorf("prediction progress or results progress cannot be empty")
	}

	var succeeded, pending, failed int
	predictions := make([]ElimPredictionEntry, 0, len(p.Progression))

	for team, predictedProgress := range p.Progression {
		resultProgress, found := results[team]

		toWin := predictedProgress.Round == "Grand Final" && predictedProgress.Status == "advanced"

		var status BucketStatus
		switch {
		case !found || resultProgress.Status == "pending":
			status = StatusPending
		case predictedProgress.Round == resultProgress.Round && predictedProgress.Status == resultProgress.Status:
			status = StatusSucceeded
		default:
			status = StatusFailed
		}

		predictions = append(predictions, ElimPredictionEntry{
			Team:   team,
			Round:  predictedProgress.Round,
			ToWin:  toWin,
			Status: status,
		})

		switch status {
		case StatusSucceeded:
			succeeded++
		case StatusPending:
			pending++
		case StatusFailed:
			failed++
		}
	}

	return SingleElimReport{
		Predictions: predictions,
		Score: models.ScoreResult{
			Successes: succeeded,
			Pending:   pending,
			Failed:    failed,
		},
	}, nil
}

// DecodeBSON unmarshals a single-elim BSON record back into an EliminationResult.
func (singleElimFormat) DecodeBSON(b []byte) (MatchResult, error) {
	var e EliminationResult
	if err := bson.Unmarshal(b, &e); err != nil {
		return nil, fmt.Errorf("single-elimination: failed to decode BSON: %w", err)
	}
	return e, nil
}

// BuildFromMatchNodes assembles an EliminationResult from parsed match nodes.
func (singleElimFormat) BuildFromMatchNodes(nodes []sources.MatchNode, round string) (MatchResult, error) {
	progression, err := getEliminationResults(nodes)
	if err != nil {
		return nil, fmt.Errorf("single-elimination: error building progression: %w", err)
	}
	return EliminationResult{Round: round, Teams: progression}, nil
}

// bracketDepth returns the number of rounds in the largest complete single-elimination
// bracket that fits within n total matches. A complete bracket of depth k has exactly
// 2^k - 1 matches (e.g. k=3 → 7 matches for QF+SF+Final).
// Extra matches (e.g. a 3rd-place consolation bout) are counted within the same depth.
//   - bracketDepth(7) = 3  (Bracket/8, no extras)
//   - bracketDepth(8) = 3  (Bracket/8 + 3rd-place match — same depth)
//   - bracketDepth(15) = 4 (Bracket/16)
func bracketDepth(n int) int {
	k := 0
	for (1<<(k+1))-1 <= n {
		k++
	}
	return k
}

// renderRoundNames maps round depth index → the section label the
// pickems-renderer recognises in its knownSingleElimOrder table.
// Index 0 = Grand Final (latest round), increasing index = earlier rounds.
var renderRoundNames = []string{
	"Grand Final",   // 0 — depth 1 bracket
	"Semifinals",    // 1 — depth 2
	"Quarterfinals", // 2 — depth 3
	"Round of 16",   // 3 — depth 4
	"Round of 32",   // 4 — depth 5
}

// NormalizeSingleElimSections rewrites the Section field on each node so the
// renderer places each match in the correct column. It is a no-op when sections
// already vary (Liquipedia already provided round-specific names). When all
// sections are identical (e.g. all "Bracket/8"), it assigns renderer-compatible
// names by match position using the same logic as getEliminationResults.
func NormalizeSingleElimSections(nodes []sources.MatchNode) []sources.MatchNode {
	if len(nodes) == 0 {
		return nodes
	}

	// No-op if sections already differ — Liquipedia gave us round names.
	allSame := true
	for i := 1; i < len(nodes); i++ {
		if nodes[i].Section != nodes[0].Section {
			allSame = false
			break
		}
	}
	if !allSame {
		return nodes
	}

	n := len(nodes)
	numRounds := int(math.Ceil(math.Log2(float64(n + 1))))

	out := make([]sources.MatchNode, len(nodes))
	copy(out, nodes)

	for i := range out {
		roundNum := roundFromIndex(i, n) // 1 = earliest (QF), numRounds = GF
		idx := numRounds - roundNum      // maps to renderRoundNames: 0 = GF
		if idx >= 0 && idx < len(renderRoundNames) {
			out[i].Section = renderRoundNames[idx]
		} else {
			out[i].Section = fmt.Sprintf("Round %d", roundNum)
		}
	}

	return out
}

// TrimSingleElimNodes trims a match node slice to the main single-elimination
// bracket by removing any extra consolation matches (e.g. a 3rd-place bout).
// A complete bracket of depth k has exactly 2^k - 1 matches; nodes beyond
// that count are dropped. Safe to call on any slice length, including empty.
func TrimSingleElimNodes(nodes []sources.MatchNode) []sources.MatchNode {
	if len(nodes) == 0 {
		return nodes
	}
	depth := bracketDepth(len(nodes))
	mainSize := (1 << depth) - 1
	if len(nodes) > mainSize {
		return nodes[:mainSize]
	}
	return nodes
}

// getEliminationResults processes a slice of match nodes and returns a map of team name → TeamProgress.
// It supports two bracket ID formats used by Liquipedia:
//   - Classic format: bracketId_R01-M001 (numeric round/match encoded in the suffix)
//   - Modern format: bracketId_RxMTP    (opaque position code — round not parseable from ID)
//
// When the classic format is detected for all matches, round numbers come from the ID.
// Otherwise, round is determined by position in the slice. LiquipediaDB returns bracket
// matches in bracket order (QF → SF → Final), so position maps cleanly to round number.
func getEliminationResults(matchNodes []sources.MatchNode) (map[string]models.TeamProgress, error) {
	if len(matchNodes) == 0 {
		return nil, fmt.Errorf("at least one match required, recieved 0")
	}

	// Trim any extra matches beyond the main bracket (e.g. a 3rd-place consolation
	// match in a Bracket/8 gives 8 total instead of 7). Extra matches fall outside
	// user predictions and would corrupt round assignments if left in.
	depth := bracketDepth(len(matchNodes))
	mainSize := (1 << depth) - 1
	if len(matchNodes) > mainSize {
		matchNodes = matchNodes[:mainSize]
	}

	// Ordered slice of rounds where [0] is the last match (grand final), and [n] is the first. This is where the limitation of 32 comes from
	rounds, err := getRoundNames(len(matchNodes))
	if err != nil {
		return nil, err
	}

	// Detect whether all IDs use the classic _R01-M001 format.
	// If any ID fails to parse, fall back to position-based round assignment.
	usePositionBased := false
	for _, m := range matchNodes {
		if _, _, err := extractRoundAndMatchIDs(m.ID); err != nil {
			usePositionBased = true
			break
		}
	}

	results := make(map[string]models.TeamProgress)

	for i, match := range matchNodes {
		var roundNum int
		if usePositionBased {
			roundNum = roundFromIndex(i, len(matchNodes))
		} else {
			roundNum, _, _ = extractRoundAndMatchIDs(match.ID)
		}

		// Safely resolve stage and rank
		round := fmt.Sprintf("Round %d", roundNum)
		rank := -1
		index := len(rounds) - roundNum
		if index >= 0 && index < len(rounds) {
			round = rounds[index]
			rank = roundNum
		}
		// Assign initial progress (pending) for each team
		for _, team := range []string{match.Team1, match.Team2} {
			if team != "" {
				existing, ok := results[team]
				if !ok || rank > getRoundIndex(existing.Round, rounds) {
					results[team] = models.TeamProgress{
						Round:  round,
						Status: "pending",
					}
				}
			}
		}

		// If there's a winner, update winner/loser status
		if match.Winner != "TBD" && match.Winner != "" {
			results[match.Winner] = models.TeamProgress{
				Round:  round,
				Status: "advanced",
			}

			// Determine loser
			var loser string
			if match.Team1 == match.Winner {
				loser = match.Team2
			} else {
				loser = match.Team1
			}
			if loser != "" {
				results[loser] = models.TeamProgress{
					Round:  round,
					Status: "eliminated",
				}
			}
		}
	}
	return results, nil
}

// getRoundIndex is a helper function to get the index of a round from its name. Used in getEliminationResults
func getRoundIndex(round string, rounds []string) int {
	for i, name := range rounds {
		if name == round {
			return len(rounds) - i
		}
	}
	return -1 // Unknown stage
}

// getRoundNames is a helper function to get the names of rounds for a single elim tournament, this is a hardcoded list with a limit of 32 matches
func getRoundNames(numMatches int) ([]string, error) {
	// Find the number of rounds, this way we can make sure the name mapping is correct
	// numRounds = log_2 (numMatches + 1) since there are always n-1 matches for a single elim tournament with n teams
	numRounds := int(math.Ceil(math.Log2(float64(numMatches + 1))))

	// Hardcoded slice of round names
	roundNames := []string{
		"Grand Final",
		"Semi Final",
		"Quarter Final",
		"Best of 16",
		"Best of 32",
	}

	if numRounds > len(roundNames) {
		return nil, fmt.Errorf("unsupported depth: only up to %d rounds supported", len(roundNames))
	}

	return roundNames[:numRounds], nil
}

// roundFromIndex returns the 1-based round number for a match at position idx
// within a bracket match slice of length total. Assumes matches are ordered
// earliest-round-first (QF → SF → Final), as LiquipediaDB returns them.
// For total=7 (Bracket/8): positions 0-3 → round 1, 4-5 → round 2, 6 → round 3.
func roundFromIndex(idx, total int) int {
	numRounds := int(math.Ceil(math.Log2(float64(total + 1))))
	cumulative := 0
	for r := 1; r <= numRounds; r++ {
		matchesInRound := 1 << (numRounds - r)
		cumulative += matchesInRound
		if idx < cumulative {
			return r
		}
	}
	return numRounds
}

// extractRoundAndMatchIDs is a helper function to get the round and match numbers from a MatchNode Id
// Id is of the form <match2bracketid>_Rxx-Myyy (e.g. RSTxQ88PoQ_R03-M001)
func extractRoundAndMatchIDs(id string) (round int, match int, err error) {
	re := regexp.MustCompile(`_R(\d+)-M(\d+)$`)
	matches := re.FindStringSubmatch(id)
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("invalid ID format: %s", id)
	}
	round, _ = strconv.Atoi(matches[1])
	match, _ = strconv.Atoi(matches[2])
	return round, match, nil
}

// setEliminationPredictions is a helper function to generate teamName : TeamProgress map used in single-elim only attributes of Prediction struct
func setEliminationPredictions(teams []string) map[string]models.TeamProgress {
	// Hard coded list of round names. We are limited to single elim brackets of size 32 due to other constraints in the project
	roundNames := []string{
		"Grand Final",
		"Semi Final",
		"Quarter Final",
		"Best of 16",
		"Best of 32",
	}
	pointer := 0

	// Input is from lowest to highest i.e. B32 -> B16 -> QF -> SF -> GF, if we reverse it the logic is a lot simpler
	slices.Reverse(teams)

	progression := make(map[string]models.TeamProgress)

	// Base case, we have to do this outside the loop because log(0) is undefined and this lets us easily set status as advanced not eliminated
	progression[teams[0]] = models.TeamProgress{Round: roundNames[pointer], Status: "advanced"}

	threshold := 1
	count := 0

	for i := 1; i <= len(teams)-1; i++ {
		// Add team to progression map
		progression[teams[i]] = models.TeamProgress{Round: roundNames[pointer], Status: "eliminated"}

		// If we reach our threshold for how many teams are in this round, we need to increment the roundName pointer and update threshold
		count++
		if count == threshold {
			pointer++
			threshold *= 2
			count = 0
		}

	}
	return progression
}
