package format

import (
	"fmt"
	"strings"

	"pickems-bot/api/external"
	"pickems-bot/api/shared"

	"go.mongodb.org/mongo-driver/bson"
)

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
	panic("format: singleElimFormat.GeneratePrediction not migrated yet (Phase 4)")
}

func (singleElimFormat) CalculateScore(p shared.Prediction, r MatchResult) (shared.ScoreResult, string, error) {
	result, ok := r.(EliminationResult)
	if !ok {
		return shared.ScoreResult{}, "", fmt.Errorf("single-elimination: expected EliminationResult, got %T", r)
	}
	results := result.Teams

	if len(p.Progression) == 0 || len(results) == 0 {
		return shared.ScoreResult{}, "", fmt.Errorf("prediction progress or results progress cannot be empty")
	}

	var succeeded, pending, failed int
	var response strings.Builder

	for team, predictedProgress := range p.Progression {
		resultProgress, found := results[team]

		if predictedProgress.Round == "Grand Final" && predictedProgress.Status == "advanced" {
			response.WriteString(fmt.Sprintf("- %s to win the %s", team, predictedProgress.Round))
		} else {
			response.WriteString(fmt.Sprintf("- %s to lose in the %s", team, predictedProgress.Round))
		}

		var status bucketStatus
		switch {
		case !found || resultProgress.Status == "pending":
			status = statusPending
		case predictedProgress.Round == resultProgress.Round && predictedProgress.Status == resultProgress.Status:
			status = statusSucceeded
		default:
			status = statusFailed
		}

		response.WriteString(fmt.Sprintf(" %s\n", status))
		switch status {
		case statusSucceeded:
			succeeded++
		case statusPending:
			pending++
		case statusFailed:
			failed++
		}
	}

	return shared.ScoreResult{
		Successes: succeeded,
		Pending:   pending,
		Failed:    failed,
	}, response.String(), nil
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
	progression, err := external.GetEliminationResults(nodes)
	if err != nil {
		return nil, fmt.Errorf("single-elimination: error building progression: %w", err)
	}
	return EliminationResult{Round: round, Teams: progression}, nil
}

func (singleElimFormat) ParseFromAPI(jsonResponse, round string) (MatchResult, error) {
	panic("format: singleElimFormat.ParseFromAPI not migrated yet (Phase 3)")
}
