/* bracket_fixture_test.go
 * Detector tests backed by real /brackets payloads captured from PandaScore,
 * to guard against the actual API shape (not just synthetic edge lists).
 * Fixtures live in testdata/.
 */

package tournament

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pickems-bot/sources"
)

func loadBracketFixture(t *testing.T, name string) []sources.BracketMatch {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	var matches []sources.BracketMatch
	require.NoError(t, json.Unmarshal(data, &matches))
	return matches
}

// A real 5-team single round-robin: edge-less (like swiss) but matchCount ==
// n*(n-1)/2, so it must classify as RoundRobin, not Swiss. Also exercises the
// full path: decode -> sources.CountTeams -> DetectKindFromBracket.
func TestDetectKindFromBracket_RealRoundRobin(t *testing.T) {
	matches := loadBracketFixture(t, "roundrobin_brackets.json")

	assert.Len(t, matches, 10, "single round-robin of 5 teams is 5*4/2 = 10 matches")

	n := sources.CountTeams(matches)
	assert.Equal(t, 5, n, "distinct teams derived from opponent IDs")

	assert.Equal(t, RoundRobin, DetectKindFromBracket(matches, n))
}
