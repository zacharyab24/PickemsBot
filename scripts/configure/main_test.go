/* main_test.go
 * Tests for the pure helper functions in the configure script.
 */

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// region pagePathFromURL

func TestPagePathFromURL_Simple(t *testing.T) {
	p, err := pagePathFromURL("https://liquipedia.net/counterstrike/PGL/2026/Bucharest")
	assert.NoError(t, err)
	assert.Equal(t, "PGL/2026/Bucharest", p)
}

func TestPagePathFromURL_TrailingSlash(t *testing.T) {
	p, err := pagePathFromURL("https://liquipedia.net/counterstrike/PGL/2026/Bucharest/")
	assert.NoError(t, err)
	assert.Equal(t, "PGL/2026/Bucharest", p)
}

func TestPagePathFromURL_StripsQueryAndFragment(t *testing.T) {
	p, err := pagePathFromURL("https://liquipedia.net/counterstrike/PGL/2026/Bucharest?foo=bar#schedule")
	assert.NoError(t, err)
	assert.Equal(t, "PGL/2026/Bucharest", p)
}

func TestPagePathFromURL_WrongHost(t *testing.T) {
	_, err := pagePathFromURL("https://example.com/counterstrike/PGL/2026/Bucharest")
	assert.Error(t, err)
}

func TestPagePathFromURL_NotCounterstrike(t *testing.T) {
	_, err := pagePathFromURL("https://liquipedia.net/dota2/Some/Page")
	assert.Error(t, err)
}

func TestPagePathFromURL_Empty(t *testing.T) {
	_, err := pagePathFromURL("https://liquipedia.net/counterstrike/")
	assert.Error(t, err)
}

func TestPagePathFromURL_Malformed(t *testing.T) {
	_, err := pagePathFromURL("://not a url")
	assert.Error(t, err)
}

// endregion

// region extractTournamentName

func TestExtractTournamentName_BasicInfobox(t *testing.T) {
	wikitext := `{{Infobox league
|name=PGL Bucharest 2026
|series=PGL Major
|liquipediatier=A-Tier
}}`
	assert.Equal(t, "PGL Bucharest 2026", extractTournamentName(wikitext))
}

func TestExtractTournamentName_LeadingWhitespace(t *testing.T) {
	wikitext := `{{Infobox league
   |  name  =  Intel Extreme Masters Cologne Major 2026
}}`
	assert.Equal(t, "Intel Extreme Masters Cologne Major 2026", extractTournamentName(wikitext))
}

func TestExtractTournamentName_NotFound(t *testing.T) {
	assert.Equal(t, "", extractTournamentName("no infobox here"))
}

// endregion

// region extractStages

func TestExtractStages_AbsoluteUnderscoreLinks(t *testing.T) {
	wikitext := `Some content [[Foo/2026/Bar/Stage_1|results]] more [[Foo/2026/Bar/Stage_2|here]]`
	stages := extractStages(wikitext, "Foo/2026/Bar")
	assert.Equal(t, []string{"Stage_1", "Stage_2"}, stages)
}

func TestExtractStages_AbsoluteSpaceLinks(t *testing.T) {
	wikitext := `[[Intel Extreme Masters/2026/Cologne/Stage 1|click here]]
[[Intel Extreme Masters/2026/Cologne/Stage 2|click here]]`
	stages := extractStages(wikitext, "Intel_Extreme_Masters/2026/Cologne")
	assert.Equal(t, []string{"Stage_1", "Stage_2"}, stages)
}

func TestExtractStages_RelativeLinks(t *testing.T) {
	wikitext := `[[/Qualifier|Closed Qualifier]]`
	stages := extractStages(wikitext, "BLAST/Bounty/2026/Summer")
	assert.Equal(t, []string{"Qualifier"}, stages)
}

func TestExtractStages_BareTemplateParams(t *testing.T) {
	// |tournament4=Foo/2026/Bar/Playoffs is a common template usage that isn't a wikilink
	wikitext := `|tournament1=#Stage 1|tournament4=Intel Extreme Masters/2026/Cologne/Playoffs`
	stages := extractStages(wikitext, "Intel_Extreme_Masters/2026/Cologne")
	assert.Equal(t, []string{"Playoffs"}, stages)
}

func TestExtractStages_SectionTransclusion(t *testing.T) {
	wikitext := `{{#section:Foo/2026/Bar/Stage 1|Results}}`
	stages := extractStages(wikitext, "Foo/2026/Bar")
	assert.Equal(t, []string{"Stage_1"}, stages)
}

func TestExtractStages_NestedCollapseToFirstSegment(t *testing.T) {
	wikitext := `[[Intel Extreme Masters/2026/Rio/Qualifier/Global|...]]
[[Intel Extreme Masters/2026/Rio/Qualifier/Americas|...]]`
	stages := extractStages(wikitext, "Intel_Extreme_Masters/2026/Rio")
	assert.Equal(t, []string{"Qualifier"}, stages)
}

func TestExtractStages_Dedupe(t *testing.T) {
	wikitext := `[[Foo/Bar/Stage_1|x]] [[Foo/Bar/Stage_1|y]] [[/Stage 1|z]]`
	stages := extractStages(wikitext, "Foo/Bar")
	assert.Equal(t, []string{"Stage_1"}, stages)
}

func TestExtractStages_None(t *testing.T) {
	stages := extractStages("nothing relevant here", "Foo/Bar")
	assert.Empty(t, stages)
}

func TestExtractStages_IgnoresAnchors(t *testing.T) {
	wikitext := `|tournament1=#Stage 1`
	stages := extractStages(wikitext, "Foo/Bar")
	assert.Empty(t, stages)
}

// endregion

// region resolveStage

func TestResolveStage_FlagOverride(t *testing.T) {
	round, page, err := resolveStage("Stage_2", []string{"Stage_1"}, "Foo/Bar", strings.NewReader(""), &bytes.Buffer{})
	assert.NoError(t, err)
	assert.Equal(t, "Stage_2", round)
	assert.Equal(t, "Foo/Bar/Stage_2", page)
}

func TestResolveStage_NoStages(t *testing.T) {
	round, page, err := resolveStage("", []string{}, "Foo/Bar", strings.NewReader(""), &bytes.Buffer{})
	assert.NoError(t, err)
	assert.Equal(t, "Main", round)
	assert.Equal(t, "Foo/Bar", page)
}

func TestResolveStage_SingleStage(t *testing.T) {
	round, page, err := resolveStage("", []string{"Qualifier"}, "Foo/Bar", strings.NewReader(""), &bytes.Buffer{})
	assert.NoError(t, err)
	assert.Equal(t, "Qualifier", round)
	assert.Equal(t, "Foo/Bar/Qualifier", page)
}

func TestResolveStage_MultiStagePicked(t *testing.T) {
	round, page, err := resolveStage("", []string{"Stage_1", "Stage_2", "Playoffs"}, "Foo/Bar", strings.NewReader("3\n"), &bytes.Buffer{})
	assert.NoError(t, err)
	assert.Equal(t, "Playoffs", round)
	assert.Equal(t, "Foo/Bar/Playoffs", page)
}

// endregion

// region pickStage

func TestPickStage_DefaultsToFirst(t *testing.T) {
	got, err := pickStage([]string{"a", "b"}, strings.NewReader("\n"), &bytes.Buffer{})
	assert.NoError(t, err)
	assert.Equal(t, "a", got)
}

func TestPickStage_ExplicitChoice(t *testing.T) {
	got, err := pickStage([]string{"a", "b", "c"}, strings.NewReader("2\n"), &bytes.Buffer{})
	assert.NoError(t, err)
	assert.Equal(t, "b", got)
}

func TestPickStage_OutOfRange(t *testing.T) {
	_, err := pickStage([]string{"a", "b"}, strings.NewReader("5\n"), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestPickStage_InvalidInput(t *testing.T) {
	_, err := pickStage([]string{"a", "b"}, strings.NewReader("not-a-number\n"), &bytes.Buffer{})
	assert.Error(t, err)
}

// endregion

// region mongoSafe

func TestMongoSafe(t *testing.T) {
	assert.Equal(t, "Intel_Extreme_Masters_Cologne_Major_2026", mongoSafe("Intel Extreme Masters Cologne Major 2026"))
	assert.Equal(t, "BLAST_tv_Austin_Major_2025", mongoSafe("BLAST.tv Austin Major 2025"))
	assert.Equal(t, "PGL_Bucharest_2026", mongoSafe("  PGL Bucharest 2026  "))
	assert.Equal(t, "X", mongoSafe("___X___"))
}

// endregion

// region writeConfig

func TestWriteConfig_Liquipedia(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.toml")

	cfg := tournamentConfig{
		DataSource: "liquipedia",
		Name:       "Foo_2026",
		Page:       "Foo/2026/Bar/Stage_1",
		Round:      "Stage_1",
		Format:     "swiss",
	}
	err := writeConfig(path, cfg)
	assert.NoError(t, err)

	data, err := os.ReadFile(path)
	assert.NoError(t, err)

	out := string(data)
	assert.True(t, strings.HasPrefix(out, "#"), "expected header comment")
	assert.Contains(t, out, `data_source     = "liquipedia"`)
	assert.Contains(t, out, `tournament_name = "Foo_2026"`)
	assert.Contains(t, out, `round           = "Stage_1"`)
	assert.Contains(t, out, `upcoming_only   = false`)
	// Nested liquipedia section
	assert.Contains(t, out, "[liquipedia]")
	assert.Contains(t, out, `api_url = "https://api.liquipedia.net/api/v3/match"`)
	assert.Contains(t, out, `page    = "Foo/2026/Bar/Stage_1"`)
	assert.Contains(t, out, `format  = "swiss"`)
	// PandaScore section always written, but api_url is blank when not the active source
	assert.Contains(t, out, "[pandascore]")
	assert.Contains(t, out, `api_url       = ""`)
	assert.Contains(t, out, "series_id     = 0")
	assert.Contains(t, out, "tournament_id = 0")
}

func TestWriteConfig_PandaScore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.toml")

	cfg := tournamentConfig{
		DataSource:   "pandascore",
		Name:         "IEM_Cologne_2026",
		SeriesID:     10488,
		TournamentID: 20708,
		Round:        "Stage_1",
	}
	err := writeConfig(path, cfg)
	assert.NoError(t, err)

	data, err := os.ReadFile(path)
	assert.NoError(t, err)

	out := string(data)
	assert.True(t, strings.HasPrefix(out, "#"), "expected header comment")
	assert.Contains(t, out, `data_source     = "pandascore"`)
	assert.Contains(t, out, `tournament_name = "IEM_Cologne_2026"`)
	assert.Contains(t, out, `round           = "Stage_1"`)
	assert.Contains(t, out, "[pandascore]")
	assert.Contains(t, out, `api_url       = "https://api.pandascore.co/csgo/matches"`)
	assert.Contains(t, out, "series_id     = 10488")
	assert.Contains(t, out, "tournament_id = 20708")
	assert.Contains(t, out, "[liquipedia]")
	assert.Contains(t, out, `api_url = ""`)
}

func TestWriteConfig_BadPath(t *testing.T) {
	err := writeConfig("/nonexistent/dir/out.toml", tournamentConfig{})
	assert.Error(t, err)
}

// endregion
