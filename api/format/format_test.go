/* format_test.go
 * Tests for the package registry.
 */

package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

// region DetectKind

func TestDetectKind_Swiss(t *testing.T) {
	wikitext := `
== Format ==
* The tournament uses a Swiss format
* Teams play best of 3 matches
`
	kind, err := DetectKind(wikitext)
	assert.NoError(t, err)
	assert.Equal(t, Swiss, kind)
}

func TestDetectKind_SingleElimination(t *testing.T) {
	wikitext := `
== Format ==
* Single-elimination bracket
* Best of 5 grand final
`
	kind, err := DetectKind(wikitext)
	assert.NoError(t, err)
	assert.Equal(t, SingleElim, kind)
}

func TestDetectKind_DoubleElimination(t *testing.T) {
	wikitext := `
== Format ==
* Double-elimination bracket
* Upper and lower bracket
`
	kind, err := DetectKind(wikitext)
	assert.NoError(t, err)
	assert.Equal(t, DoubleElim, kind)
}

func TestDetectKind_BothFormatsPrefersSwiss(t *testing.T) {
	wikitext := `
== Format ==
* Swiss stage followed by single-elimination playoffs
`
	kind, err := DetectKind(wikitext)
	assert.NoError(t, err)
	assert.Equal(t, Swiss, kind)
}

func TestDetectKind_CaseInsensitive(t *testing.T) {
	wikitext := `
== Format ==
* SWISS FORMAT tournament
`
	kind, err := DetectKind(wikitext)
	assert.NoError(t, err)
	assert.Equal(t, Swiss, kind)
}

func TestDetectKind_NoFormatSection(t *testing.T) {
	wikitext := `
== Tournament ==
* Some tournament info
`
	_, err := DetectKind(wikitext)
	assert.Error(t, err)
}

func TestDetectKind_UnrecognizedFormat(t *testing.T) {
	wikitext := `
== Format ==
* Round-robin format
`
	_, err := DetectKind(wikitext)
	assert.Error(t, err)
}

// endregion
