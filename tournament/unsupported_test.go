package tournament

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pickems-bot/models"
)

func TestUnsupportedFormat_ResolvesForUnscoreableKinds(t *testing.T) {
	// Get must succeed for these kinds so non-prediction paths resolve the format
	// cleanly instead of erroring on an unregistered kind.
	for _, k := range []Kind{Other, DoubleElim} {
		f, err := Get(k)
		require.NoError(t, err, "kind %q should resolve to the unsupported format", k)
		assert.Equal(t, k, f.Name())
	}
}

func TestUnsupportedFormat_PredictionMethodsRefuse(t *testing.T) {
	f := MustGet(Other)

	assert.Equal(t, 0, f.RequiredPredictions(16))

	_, err := f.GeneratePrediction(models.User{}, "Playoffs", []string{"A"})
	assert.ErrorIs(t, err, ErrPredictionsUnsupported)

	_, err = f.CalculateScore(models.Prediction{}, nil)
	assert.ErrorIs(t, err, ErrPredictionsUnsupported)

	_, err = f.PredictionFields(models.Prediction{})
	assert.ErrorIs(t, err, ErrPredictionsUnsupported)

	_, err = f.BuildFromMatchNodes(nil, "Playoffs")
	assert.ErrorIs(t, err, ErrPredictionsUnsupported)

	_, err = f.DecodeBSON(nil)
	assert.ErrorIs(t, err, ErrPredictionsUnsupported)
}
