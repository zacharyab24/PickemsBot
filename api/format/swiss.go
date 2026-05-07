package format

import (
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
)

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

func (swissFormat) GeneratePrediction(user shared.User, round string, teams []string) (store.Prediction, error) {
	panic("format: swissFormat.GeneratePrediction not migrated yet (Phase 4)")
}

func (swissFormat) CalculateScore(p store.Prediction, r external.MatchResult) (store.ScoreResult, string, error) {
	panic("format: swissFormat.CalculateScore not migrated yet (Phase 1)")
}

func (swissFormat) ToRecord(r external.MatchResult, round string) store.ResultRecord {
	panic("format: swissFormat.ToRecord not migrated yet (Phase 2)")
}

func (swissFormat) FromRecord(rec store.ResultRecord) (external.MatchResult, error) {
	panic("format: swissFormat.FromRecord not migrated yet (Phase 2)")
}

func (swissFormat) ParseFromAPI(json string) (external.MatchResult, error) {
	panic("format: swissFormat.ParseFromAPI not migrated yet (Phase 3)")
}
