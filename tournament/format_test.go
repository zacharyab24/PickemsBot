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

func TestSupportsPredictions_SwissAndSingleElimTrue(t *testing.T) {
	assert.True(t, MustGet(Swiss).SupportsPredictions())
	assert.True(t, MustGet(SingleElim).SupportsPredictions())
}

func TestSupportsPredictions_UnsupportedKindsFalse(t *testing.T) {
	assert.False(t, MustGet(DoubleElim).SupportsPredictions())
	assert.False(t, MustGet(RoundRobin).SupportsPredictions())
	assert.False(t, MustGet(Other).SupportsPredictions())
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

func TestDetectKindFromMatchNodes_NoMatchingSections_FallsBackToOther(t *testing.T) {
	kind, err := DetectKindFromMatchNodes(nodes("Group A", "Group B"))
	assert.NoError(t, err)
	assert.Equal(t, Other, kind)
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

// region DetectKindFromBracket

// bm builds a BracketMatch whose previous_matches carry the given edge types.
// Called with no arguments it is an entry-point match (empty previous_matches),
// exactly how PandaScore represents an upper-round-1 / quarterfinal match.
func bm(edgeTypes ...string) sources.BracketMatch {
	var m sources.BracketMatch
	for i, typ := range edgeTypes {
		m.PreviousMatches = append(m.PreviousMatches, sources.BracketEdge{FromMatchID: i + 1, Type: typ})
	}
	return m
}

// edgelessBracket returns n entry-point matches (no feeder edges), the shape a
// group stage (swiss / round-robin) has - pairings come from standings, not
// from prior match outcomes.
func edgelessBracket(n int) []sources.BracketMatch {
	return make([]sources.BracketMatch, n)
}

func TestDetectKindFromBracket_DoubleElim(t *testing.T) {
	// Entry matches with no edges, a winner-fed upper match, at least one
	// loser-fed lower-bracket match, and a winner-fed grand final.
	matches := []sources.BracketMatch{
		bm(), bm(),
		bm("winner", "winner"),
		bm("loser", "winner"),
		bm("winner", "winner"),
	}
	assert.Equal(t, DoubleElim, DetectKindFromBracket(matches, 8))
}

func TestDetectKindFromBracket_SingleElim(t *testing.T) {
	// 8-team single elim: 4 entry-point quarterfinals, 2 semis, 1 final; winner edges only.
	matches := []sources.BracketMatch{
		bm(), bm(), bm(), bm(),
		bm("winner", "winner"), bm("winner", "winner"),
		bm("winner", "winner"),
	}
	assert.Equal(t, SingleElim, DetectKindFromBracket(matches, 8))
}

func TestDetectKindFromBracket_Swiss(t *testing.T) {
	// 16-team swiss, 33 matches, no feeder edges. 33 != 16*15/2 (=120), so not round-robin.
	assert.Equal(t, Swiss, DetectKindFromBracket(edgelessBracket(33), 16))
}

func TestDetectKindFromBracket_RoundRobin(t *testing.T) {
	// 4 teams, 6 matches (== 4*3/2), no edges -> round-robin (no scorer, but its own kind).
	assert.Equal(t, RoundRobin, DetectKindFromBracket(edgelessBracket(6), 4))
}

func TestDetectKindFromBracket_EmptyIsOther(t *testing.T) {
	assert.Equal(t, Other, DetectKindFromBracket(nil, 8))
	assert.Equal(t, Other, DetectKindFromBracket([]sources.BracketMatch{}, 8))
}

func TestDetectKindFromBracket_LoserEdgeWinsOverWinnerEdges(t *testing.T) {
	// A single loser edge means a lower bracket exists -> double-elim, even amid
	// many winner edges. Guards the priority ordering.
	matches := []sources.BracketMatch{
		bm("winner", "winner"),
		bm("winner", "winner"),
		bm("loser", "winner"),
	}
	assert.Equal(t, DoubleElim, DetectKindFromBracket(matches, 8))
}

func TestDetectKindFromBracket_EntryPointMatchesDoNotForceGroup(t *testing.T) {
	// Edge-less entry matches appearing before any edged match must not short-circuit
	// to a group classification: the scan is over the whole set, order-independent.
	matches := []sources.BracketMatch{
		bm(), bm(), bm(),
		bm("winner", "winner"),
	}
	assert.Equal(t, SingleElim, DetectKindFromBracket(matches, 8))
}

func TestDetectKindFromBracket_EdgeTypeCaseInsensitive(t *testing.T) {
	assert.Equal(t, DoubleElim, DetectKindFromBracket([]sources.BracketMatch{bm("LOSER")}, 8))
	assert.Equal(t, SingleElim, DetectKindFromBracket([]sources.BracketMatch{bm("Winner")}, 8))
}

// endregion

// region isRoundRobin

func TestIsRoundRobin(t *testing.T) {
	assert.True(t, isRoundRobin(6, 4))    // 4 teams -> 6 matches
	assert.True(t, isRoundRobin(10, 5))   // 5 teams -> 10 matches
	assert.False(t, isRoundRobin(33, 16)) // swiss, far short of the full 120
	assert.False(t, isRoundRobin(5, 4))   // wrong count for 4 teams
	assert.False(t, isRoundRobin(0, 1))   // single team guarded out
	assert.False(t, isRoundRobin(0, 0))   // no teams
}

// endregion
