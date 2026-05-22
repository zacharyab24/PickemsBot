package format

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"

	"pickems-bot/api/external"
	"pickems-bot/api/shared"

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
	Score       shared.ScoreResult
}

// FormatKind implements ScoreReport.
func (SingleElimReport) FormatKind() Kind { return SingleElim }

// GetScore implements ScoreReport.
func (s SingleElimReport) GetScore() shared.ScoreResult { return s.Score }

// EliminationResult is the unified in-memory + on-disk representation of a
// single-elimination bracket's progression data. Teams maps team name →
// TeamProgress (round reached + advanced/eliminated/pending status).
type EliminationResult struct {
	Round string                         `bson:"round,omitempty"`
	Teams map[string]shared.TeamProgress `bson:"teams,omitempty"`
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

func (singleElimFormat) GeneratePrediction(user shared.User, round string, teams []string) (shared.Prediction, error) {
	// Set generic attributes for Prediction struct
	prediction := shared.Prediction{
		UserID:   user.UserID,
		Username: user.Username,
		Format:   "single-elimination",
		Round:    round,
	}

	progression := setEliminationPredictions(teams)
	prediction.Progression = progression

	return prediction, nil
}

func (singleElimFormat) CalculateScore(p shared.Prediction, r MatchResult) (ScoreReport, error) {
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
		Score: shared.ScoreResult{
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
// Phase 3 will inline external.GetEliminationResults into this package.
func (singleElimFormat) BuildFromMatchNodes(nodes []external.MatchNode, round string) (MatchResult, error) {
	progression, err := getEliminationResults(nodes)
	if err != nil {
		return nil, fmt.Errorf("single-elimination: error building progression: %w", err)
	}
	return EliminationResult{Round: round, Teams: progression}, nil
}

// ExtractMatchListID parses wikitext and extracts the `Matchlist` id
func (singleElimFormat) ExtractMatchListIDs(wikitext string) ([]string, Kind, error) {
	re := regexp.MustCompile(`(?s)\{\{\s*(?:Show)?Bracket\s*\|([^}]*)\}\}`) // {{Bracket ...}} and {{ShowBracket ...}} templates used in single elim
	return extractMatchListIds(wikitext, re)

}

// getEliminationResults processs a slice of match nodes and return a map of team name : TeamProgress. Called by BuildMatchNodes
func getEliminationResults(matchNodes []external.MatchNode) (map[string]shared.TeamProgress, error) {
	if len(matchNodes) == 0 {
		return nil, fmt.Errorf("at least one match required, recieved 0")
	}

	// Ordered slice of rounds where [0] is the last match (grand final), and [n] is the first. This is where the limitation of 32 comes from
	rounds, err := getRoundNames(len(matchNodes))
	if err != nil {
		return nil, err
	}

	results := make(map[string]shared.TeamProgress)

	for _, match := range matchNodes {
		roundNum, _, err := extractRoundAndMatchIDs(match.ID)
		if err != nil {
			return nil, err
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
					results[team] = shared.TeamProgress{
						Round:  round,
						Status: "pending",
					}
				}
			}
		}

		// If there's a winner, update winner/loser status
		if match.Winner != "TBD" && match.Winner != "" {
			results[match.Winner] = shared.TeamProgress{
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
				results[loser] = shared.TeamProgress{
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
func setEliminationPredictions(teams []string) map[string]shared.TeamProgress {
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

	progression := make(map[string]shared.TeamProgress)

	// Base case, we have to do this outside the loop because log(0) is undefined and this lets us easily set status as advanced not eliminated
	progression[teams[0]] = shared.TeamProgress{Round: roundNames[pointer], Status: "advanced"}

	threshold := 1
	count := 0

	for i := 1; i <= len(teams)-1; i++ {
		// Add team to progression map
		progression[teams[i]] = shared.TeamProgress{Round: roundNames[pointer], Status: "eliminated"}

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
