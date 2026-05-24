//go:build integration

package sources_test

import (
	"testing"

	"pickems-bot/sources"
	"pickems-bot/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	liqURL     = testhelpers.TestServerBase + "/liquipedia/api/v3/match"
	liqTestKey = "pickems-test-key"
	liqPage    = "Test/Tournament/2026"
)

// TestLiquipedia_GetMatches_Ongoing verifies that the ongoing state returns
// a non-empty list of well-formed matches.
func TestLiquipedia_GetMatches_Ongoing(t *testing.T) {
	testhelpers.SetState(t, "liquipedia", "ongoing")

	raw, err := sources.GetLiquipediaMatchDataByPage(liqURL, liqTestKey, liqPage)
	require.NoError(t, err)

	nodes, err := sources.ParseLiquipediaMatches(raw)
	require.NoError(t, err)
	assert.NotEmpty(t, nodes, "expected matches in ongoing state")

	for _, n := range nodes {
		assert.NotEmpty(t, n.ID)
		assert.NotEmpty(t, n.Team1)
		assert.NotEmpty(t, n.Team2)
	}
}

// TestLiquipedia_GetMatches_NotStarted verifies that the not_started state
// returns an empty result set without an error (bot must not crash).
func TestLiquipedia_GetMatches_NotStarted(t *testing.T) {
	testhelpers.SetState(t, "liquipedia", "not_started")

	raw, err := sources.GetLiquipediaMatchDataByPage(liqURL, liqTestKey, liqPage)
	require.NoError(t, err)

	nodes, err := sources.ParseLiquipediaMatches(raw)
	require.NoError(t, err)
	assert.Empty(t, nodes, "expected empty match list in not_started state")
}

// TestLiquipedia_GetMatches_WrongKey verifies that a bad API key returns an
// error (403 from the test server).
func TestLiquipedia_GetMatches_WrongKey(t *testing.T) {
	testhelpers.SetState(t, "liquipedia", "ongoing")

	_, err := sources.GetLiquipediaMatchDataByPage(liqURL, "wrong-key", liqPage)
	require.Error(t, err, "expected error for bad API key")
}
