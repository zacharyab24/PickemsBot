//go:build integration

package sources_test

import (
	"errors"
	"testing"

	"pickems-bot/sources"
	"pickems-bot/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	psURL      = testhelpers.TestServerBase + "/pandascore/csgo/matches"
	psTestKey  = "pickems-test-key"
	psSeriesID = 99001
)

// TestPandaScore_GetMatches_Ongoing verifies that the ongoing state returns
// a non-empty list of well-formed matches.
func TestPandaScore_GetMatches_Ongoing(t *testing.T) {
	testhelpers.SetState(t, "pandascore", "ongoing")

	raw, err := sources.GetPandaScoreMatches(psURL, psTestKey, psSeriesID, 0)
	require.NoError(t, err)

	nodes, err := sources.ParsePandaScoreMatches(raw, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, nodes, "expected matches in ongoing state")

	for _, n := range nodes {
		assert.NotEmpty(t, n.ID)
		assert.NotEmpty(t, n.Team1)
		assert.NotEmpty(t, n.Team2)
	}
}

// TestPandaScore_GetMatches_NotStarted verifies that the not_started state
// returns an empty list without an error (the bot must not crash).
func TestPandaScore_GetMatches_NotStarted(t *testing.T) {
	testhelpers.SetState(t, "pandascore", "not_started")

	raw, err := sources.GetPandaScoreMatches(psURL, psTestKey, psSeriesID, 0)
	require.NoError(t, err)

	nodes, err := sources.ParsePandaScoreMatches(raw, 0)
	require.NoError(t, err)
	assert.Empty(t, nodes, "expected empty match list in not_started state")
}

// TestPandaScore_GetMatches_WrongKey verifies that a bad API key causes an
// ErrUnrecoverable error (401 from the test server).
func TestPandaScore_GetMatches_WrongKey(t *testing.T) {
	testhelpers.SetState(t, "pandascore", "ongoing")

	_, err := sources.GetPandaScoreMatches(psURL, "wrong-key", psSeriesID, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, sources.ErrUnrecoverable),
		"expected ErrUnrecoverable for bad API key, got: %v", err)
}

// TestPandaScore_GetMatches_NotFound verifies that an unknown series ID causes
// an ErrUnrecoverable error (404 from the test server).
func TestPandaScore_GetMatches_NotFound(t *testing.T) {
	// The test server returns 404 when filter[serie_id] is present but not "99001".
	// Use any other value to trigger this.
	_, err := sources.GetPandaScoreMatches(psURL, psTestKey, 12345, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, sources.ErrUnrecoverable),
		"expected ErrUnrecoverable for unknown series ID, got: %v", err)
}

// TestPandaScore_Complete_HasMoreFinished verifies that the complete state has
// at least one more finished match than the ongoing state.
func TestPandaScore_Complete_HasMoreFinished(t *testing.T) {
	testhelpers.SetState(t, "pandascore", "ongoing")
	rawOngoing, err := sources.GetPandaScoreMatches(psURL, psTestKey, psSeriesID, 0)
	require.NoError(t, err)
	ongoingNodes, err := sources.ParsePandaScoreMatches(rawOngoing, 0)
	require.NoError(t, err)

	testhelpers.SetState(t, "pandascore", "complete")
	rawComplete, err := sources.GetPandaScoreMatches(psURL, psTestKey, psSeriesID, 0)
	require.NoError(t, err)
	completeNodes, err := sources.ParsePandaScoreMatches(rawComplete, 0)
	require.NoError(t, err)

	countFinished := func(nodes []sources.MatchNode) int {
		n := 0
		for _, m := range nodes {
			if m.Status == "finished" {
				n++
			}
		}
		return n
	}

	assert.Greater(t, countFinished(completeNodes), countFinished(ongoingNodes),
		"complete state should have more finished matches than ongoing")
}
