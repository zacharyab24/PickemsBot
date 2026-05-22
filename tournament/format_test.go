/* format_test.go
 * Tests for the package registry.
 */

package tournament

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pickems-bot/sources"
)

func TestGet_ReturnsRegisteredFormats(t *testing.T) {
	swiss, err := Get(Swiss)
	assert.NoError(t, err)
	assert.Equal(t, Swiss, swiss.Name())

	se, err := Get(SingleElim)
	assert.NoError(t, err)
	assert.Equal(t, SingleElim, se.Name())
}

func TestGet_UnknownReturnsError(t *testing.T) {
	_, err := Get("does-not-exist")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}

func TestMustGet_ReturnsRegisteredFormat(t *testing.T) {
	assert.Equal(t, Swiss, MustGet(Swiss).Name())
}

func TestMustGet_PanicsOnUnknown(t *testing.T) {
	assert.Panics(t, func() { MustGet("does-not-exist") })
}

func TestNames_ContainsRegisteredFormats(t *testing.T) {
	names := Names()
	assert.Contains(t, names, Swiss)
	assert.Contains(t, names, SingleElim)
}

func TestRegister_PanicsOnDuplicate(t *testing.T) {
	assert.Panics(t, func() { register(swissFormat{}) })
}

// region DetectKindFromMatchNodes

func nodes(sections ...string) []sources.MatchNode {
	out := make([]sources.MatchNode, len(sections))
	for i, s := range sections {
		out[i] = sources.MatchNode{Section: s}
	}
	return out
}

func TestDetectKindFromMatchNodes_Swiss(t *testing.T) {
	kind, err := DetectKindFromMatchNodes(nodes("Round 1", "Round 2", "Round 3"))
	assert.NoError(t, err)
	assert.Equal(t, Swiss, kind)
}

func TestDetectKindFromMatchNodes_SingleElim(t *testing.T) {
	kind, err := DetectKindFromMatchNodes(nodes("Quarterfinal", "Semifinal", "Grand Final"))
	assert.NoError(t, err)
	assert.Equal(t, SingleElim, kind)
}

func TestDetectKindFromMatchNodes_DoubleElim(t *testing.T) {
	kind, err := DetectKindFromMatchNodes(nodes("Upper Bracket Round 1", "Lower Bracket Round 1", "Grand Final"))
	assert.NoError(t, err)
	assert.Equal(t, DoubleElim, kind)
}

func TestDetectKindFromMatchNodes_DoubleElimTakesPriorityOverSwiss(t *testing.T) {
	// "lower" keyword present alongside "round" — should resolve to DoubleElim
	kind, err := DetectKindFromMatchNodes(nodes("Upper Bracket Round 1", "Lower Bracket Round 1"))
	assert.NoError(t, err)
	assert.Equal(t, DoubleElim, kind)
}

func TestDetectKindFromMatchNodes_CaseInsensitive(t *testing.T) {
	kind, err := DetectKindFromMatchNodes(nodes("ROUND 1", "ROUND 2"))
	assert.NoError(t, err)
	assert.Equal(t, Swiss, kind)
}

func TestDetectKindFromMatchNodes_NoMatchingSections(t *testing.T) {
	_, err := DetectKindFromMatchNodes(nodes("Group A", "Group B"))
	assert.Error(t, err)
}

func TestDetectKindFromMatchNodes_EmptyNodes(t *testing.T) {
	_, err := DetectKindFromMatchNodes([]sources.MatchNode{})
	assert.Error(t, err)
}

// endregion

// region FilterNodesByKind

func TestFilterNodesByKind_SwissKeepsOnlyRoundSections(t *testing.T) {
	input := nodes("Round 1", "Round 2", "Playoffs", "Showmatch", "Grand Final")
	got := FilterNodesByKind(input, Swiss)
	assert.Len(t, got, 2)
	assert.Equal(t, "Round 1", got[0].Section)
	assert.Equal(t, "Round 2", got[1].Section)
}

func TestFilterNodesByKind_SwissCaseInsensitive(t *testing.T) {
	input := nodes("ROUND 1", "round 2", "Bracket Stage")
	got := FilterNodesByKind(input, Swiss)
	assert.Len(t, got, 2)
}

func TestFilterNodesByKind_SwissEmptyResult(t *testing.T) {
	input := nodes("Playoffs", "Grand Final", "Showmatch")
	got := FilterNodesByKind(input, Swiss)
	assert.Empty(t, got)
}

func TestFilterNodesByKind_SingleElimKeepsBracketSections(t *testing.T) {
	input := nodes("Quarterfinal", "Semifinal", "Grand Final")
	got := FilterNodesByKind(input, SingleElim)
	assert.Len(t, got, 3)
}

func TestFilterNodesByKind_SingleElimLiquipediaBracketTemplate(t *testing.T) {
	// Liquipedia bracket templates use section names like "Bracket/8", "Bracket/4"
	input := nodes("Bracket/8", "Bracket/4", "Bracket/2", "Bracket/1")
	got := FilterNodesByKind(input, SingleElim)
	assert.Len(t, got, 4)
}

func TestFilterNodesByKind_SingleElimMixedPage(t *testing.T) {
	// PGL-Major-style page: Swiss rounds + playoffs bracket + showmatch
	input := nodes("Round 1", "Round 2", "Round 3", "Bracket/8", "Showmatch")
	got := FilterNodesByKind(input, SingleElim)
	assert.Len(t, got, 1)
	assert.Equal(t, "Bracket/8", got[0].Section)
}

func TestFilterNodesByKind_SingleElimPlayoffsKeyword(t *testing.T) {
	input := nodes("Playoffs", "Round 1", "Showmatch")
	got := FilterNodesByKind(input, SingleElim)
	assert.Len(t, got, 1)
	assert.Equal(t, "Playoffs", got[0].Section)
}

func TestFilterNodesByKind_DoubleElimPassesThrough(t *testing.T) {
	input := nodes("Upper Bracket Round 1", "Lower Bracket Round 1", "Grand Final")
	got := FilterNodesByKind(input, DoubleElim)
	assert.Equal(t, input, got)
}

// endregion
