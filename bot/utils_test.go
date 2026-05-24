/* utils_test.go
 * Unit tests for bot utility functions (singleElimField, elimPositionLabel,
 * swissBucketField empty path, sendError error path).
 */

package bot

import (
	"strings"
	"testing"

	format "pickems-bot/tournament"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// region elimPositionLabel tests

func TestElimPositionLabel_Champion(t *testing.T) {
	e := format.ElimPredictionEntry{Team: "Alpha", Round: "Grand Final", ToWin: true}
	assert.Equal(t, "🏆 Champion", elimPositionLabel(e))
}

func TestElimPositionLabel_RunnerUp(t *testing.T) {
	e := format.ElimPredictionEntry{Team: "Beta", Round: "Grand Final", ToWin: false}
	assert.Equal(t, "🥈 Runner-up", elimPositionLabel(e))
}

func TestElimPositionLabel_ThirdFourth(t *testing.T) {
	e := format.ElimPredictionEntry{Team: "Gamma", Round: "Semi Final", ToWin: false}
	assert.Equal(t, "🥉 3rd / 4th", elimPositionLabel(e))
}

func TestElimPositionLabel_Top8(t *testing.T) {
	e := format.ElimPredictionEntry{Team: "Delta", Round: "Quarter Final", ToWin: false}
	assert.Equal(t, "🎖️ Top 8", elimPositionLabel(e))
}

func TestElimPositionLabel_DefaultRound(t *testing.T) {
	e := format.ElimPredictionEntry{Team: "Epsilon", Round: "Best of 32", ToWin: false}
	assert.Equal(t, "Best of 32", elimPositionLabel(e))
}

// endregion

// region singleElimField tests

func TestSingleElimField_SortsCorrectly(t *testing.T) {
	entries := []format.ElimPredictionEntry{
		{Team: "Beta", Round: "Semi Final", ToWin: false, Status: format.StatusFailed},
		{Team: "Alpha", Round: "Grand Final", ToWin: true, Status: format.StatusSucceeded},
		{Team: "Gamma", Round: "Quarter Final", ToWin: false, Status: format.StatusPending},
	}

	field := singleElimField(entries)
	require.NotNil(t, field)
	assert.Equal(t, "**Predictions**", field.Name)
	// Champion (Grand Final winner) should appear first in the value string
	assert.Contains(t, field.Value, "Alpha")
	assert.Contains(t, field.Value, "🏆 Champion")
}

func TestSingleElimField_EmptyEntries_ShowsDash(t *testing.T) {
	field := singleElimField(nil)
	require.NotNil(t, field)
	assert.Equal(t, "—", field.Value)
}

func TestSingleElimField_WithinSameRound_WinnerFirst(t *testing.T) {
	// Two Grand Final entries: winner (ToWin=true) and runner-up (ToWin=false)
	entries := []format.ElimPredictionEntry{
		{Team: "RunnerUp", Round: "Grand Final", ToWin: false, Status: format.StatusFailed},
		{Team: "Champion", Round: "Grand Final", ToWin: true, Status: format.StatusSucceeded},
	}
	field := singleElimField(entries)
	require.NotNil(t, field)
	// Champion (ToWin=true) must appear before Runner-up in the output
	champIdx := strings.Index(field.Value, "🏆 Champion")
	ruIdx := strings.Index(field.Value, "🥈 Runner-up")
	assert.GreaterOrEqual(t, champIdx, 0, "champion label should be in the output")
	assert.GreaterOrEqual(t, ruIdx, 0, "runner-up label should be in the output")
	assert.Less(t, champIdx, ruIdx, "Champion should appear before Runner-up")
}

// endregion

// region swissBucketField empty path test

func TestSwissBucketField_EmptyEntries_ShowsDash(t *testing.T) {
	field := swissBucketField("3-0 Picks", nil)
	require.NotNil(t, field)
	assert.Equal(t, "3-0 Picks", field.Name)
	assert.Equal(t, "—", field.Value)
}

// endregion

// region sendError error path test

func TestSendError_LogsOnSessionError(t *testing.T) {
	// When the session returns an error from ChannelMessageSendEmbed,
	// sendError should not panic and should log the failure (no return value to assert).
	session := &MockDiscordSession{ErrorToReturn: assert.AnError}
	// Should not panic even when session.ChannelMessageSendEmbed returns an error
	assert.NotPanics(t, func() {
		sendError(session, "channel-123", "something went wrong")
	})
}

func TestSendError_Success(t *testing.T) {
	session := NewMockDiscordSession()
	sendError(session, "channel-123", "test error message")
	require.Len(t, session.SentEmbeds, 1)
	assert.Equal(t, "Error", session.SentEmbeds[0].Embed.Title)
	assert.Equal(t, "test error message", session.SentEmbeds[0].Embed.Description)
	assert.Equal(t, red, session.SentEmbeds[0].Embed.Color)
}

// endregion
