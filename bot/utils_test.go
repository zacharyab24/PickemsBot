package bot

import (
	"testing"

	format "pickems-bot/tournament"

	"github.com/stretchr/testify/assert"
)

func TestElimPositionLabel(t *testing.T) {
	tests := []struct {
		name  string
		entry format.ElimPredictionEntry
		want  string
	}{
		{"champion", format.ElimPredictionEntry{Round: "Grand Final", ToWin: true}, "🏆 Champion"},
		{"runner-up", format.ElimPredictionEntry{Round: "Grand Final"}, "🥈 Runner-up"},
		{"semi-finalist", format.ElimPredictionEntry{Round: "Semi Final"}, "🥉 3rd / 4th"},
		{"quarter-finalist", format.ElimPredictionEntry{Round: "Quarter Final"}, "🎖️ Top 8"},
		{"earlier round falls back to round name", format.ElimPredictionEntry{Round: "Best of 32"}, "Best of 32"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, elimPositionLabel(tt.entry))
		})
	}
}

func TestSingleElimField_OrdersChampionFirstThenByRound(t *testing.T) {
	entries := []format.ElimPredictionEntry{
		{Team: "Gamma", Round: "Semi Final", Status: format.StatusPending},
		{Team: "Beta", Round: "Grand Final", Status: format.StatusFailed},
		{Team: "Delta", Round: "Semi Final", Status: format.StatusPending},
		{Team: "Alpha", Round: "Grand Final", ToWin: true, Status: format.StatusSucceeded},
	}

	field := singleElimField(entries)

	assert.Equal(t, "**Predictions**", field.Name)
	assert.Equal(t,
		"🏆 Champion: **Alpha** ✅\n"+
			"🥈 Runner-up: **Beta** ❌\n"+
			"🥉 3rd / 4th: **Delta** ⏳\n"+
			"🥉 3rd / 4th: **Gamma** ⏳\n",
		field.Value)
}

func TestSingleElimField_Empty_ShowsDash(t *testing.T) {
	assert.Equal(t, "—", singleElimField(nil).Value)
}

func TestSwissBucketField_ShowsRecordOrNA(t *testing.T) {
	field := swissBucketField("**3-0**", []format.BucketEntry{
		{Team: "Alpha", Score: "3-0", Status: format.StatusSucceeded},
		{Team: "Beta", Score: "", Status: format.StatusPending},
	})

	assert.Equal(t, "**3-0**", field.Name)
	assert.Equal(t, "**Alpha**: 3-0 ✅\n**Beta**: N/A ⏳\n", field.Value)
}

func TestSwissBucketField_Empty_ShowsDash(t *testing.T) {
	assert.Equal(t, "—", swissBucketField("**3-0**", nil).Value)
}
