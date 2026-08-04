package tournament

import (
	"errors"

	"pickems-bot/models"
	"pickems-bot/sources"
)

// ErrPredictionsUnsupported is returned by every prediction/scoring method of a
// format that the bot can catalog and show raw match data for, but cannot run
// pick'ems on (double-elimination, or an unrecognised bracket classified as
// "other"). Callers should surface a user-facing "predictions not supported"
// message rather than treat it as an internal failure.
var ErrPredictionsUnsupported = errors.New("tournament format does not support predictions")

// unsupportedFormat is a null-object Format registered for the kinds we detect
// but can't run predictions for. Registering it (instead of leaving Get to error)
// is deliberate: non-prediction features (schedule, raw results, future
// read-only commands) resolve the format cleanly, while every prediction/scoring
// entry point returns ErrPredictionsUnsupported and does no work. Add a real
// implementation later to promote a kind out of "unsupported".
type unsupportedFormat struct{ kind Kind }

var _ Format = unsupportedFormat{}

func init() {
	register(unsupportedFormat{kind: DoubleElim})
	register(unsupportedFormat{kind: Other})
}

func (f unsupportedFormat) Name() Kind { return f.kind }

// RequiredPredictions is 0: there are no predictions to make for this format.
func (unsupportedFormat) RequiredPredictions(int) int { return 0 }

func (unsupportedFormat) GeneratePrediction(models.User, string, []string) (models.Prediction, error) {
	return models.Prediction{}, ErrPredictionsUnsupported
}

func (unsupportedFormat) CalculateScore(models.Prediction, MatchResult) (ScoreReport, error) {
	return nil, ErrPredictionsUnsupported
}

func (unsupportedFormat) PredictionFields(models.Prediction) ([]models.PredictionField, error) {
	return nil, ErrPredictionsUnsupported
}

func (unsupportedFormat) DecodeBSON([]byte) (MatchResult, error) {
	return nil, ErrPredictionsUnsupported
}

func (unsupportedFormat) BuildFromMatchNodes([]sources.MatchNode, string) (MatchResult, error) {
	return nil, ErrPredictionsUnsupported
}
