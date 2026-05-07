package format

import (
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
)

// singleElimFormat implements Format for single-elimination bracket tournaments.
type singleElimFormat struct{}

var _ Format = singleElimFormat{}

func init() { register(singleElimFormat{}) }

func (singleElimFormat) Name() Kind { return SingleElim }

// RequiredPredictions returns teamCount / 2 — one pick per first-round
// matchup, predicting which team advances.
func (singleElimFormat) RequiredPredictions(teamCount int) int { return teamCount / 2 }

func (singleElimFormat) GeneratePrediction(user shared.User, round string, teams []string) (store.Prediction, error) {
	panic("format: singleElimFormat.GeneratePrediction not migrated yet (Phase 4)")
}

func (singleElimFormat) CalculateScore(p store.Prediction, r external.MatchResult) (store.ScoreResult, string, error) {
	panic("format: singleElimFormat.CalculateScore not migrated yet (Phase 1)")
}

func (singleElimFormat) ToRecord(r external.MatchResult, round string) store.ResultRecord {
	panic("format: singleElimFormat.ToRecord not migrated yet (Phase 2)")
}

func (singleElimFormat) FromRecord(rec store.ResultRecord) (external.MatchResult, error) {
	panic("format: singleElimFormat.FromRecord not migrated yet (Phase 2)")
}

func (singleElimFormat) ParseFromAPI(json string) (external.MatchResult, error) {
	panic("format: singleElimFormat.ParseFromAPI not migrated yet (Phase 3)")
}
