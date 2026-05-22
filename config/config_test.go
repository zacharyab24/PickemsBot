/* config_test.go
 * Tests for the config loader.
 */

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeTemp(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}

func TestLoad_AllFields_Liquipedia(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "MyEvent_2026"
data_source = "liquipedia"
page = "Foo/2026/Bar/Stage_1"
round = "Stage_1"
upcoming_only = true
test = true
`)

	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.Equal(t, "MyEvent_2026", cfg.TournamentName)
	assert.Equal(t, "liquipedia", cfg.DataSource)
	assert.Equal(t, "Foo/2026/Bar/Stage_1", cfg.Page)
	assert.Equal(t, "Stage_1", cfg.Round)
	assert.True(t, cfg.UpcomingOnly)
	assert.True(t, cfg.Test)
}

func TestLoad_AllFields_PandaScore(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "MyEvent_2026"
data_source = "pandascore"
series_id = 10583
round = "Stage_1"
upcoming_only = true
test = true
`)

	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.Equal(t, "MyEvent_2026", cfg.TournamentName)
	assert.Equal(t, "pandascore", cfg.DataSource)
	assert.Equal(t, 10583, cfg.SeriesId)
	assert.Equal(t, "Stage_1", cfg.Round)
	assert.True(t, cfg.UpcomingOnly)
	assert.True(t, cfg.Test)
}

func TestLoad_DefaultBoolsAndOptionalParams(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "liquipedia"
page = "Y/Z"
round = "Z"
`)

	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.False(t, cfg.UpcomingOnly)
	assert.False(t, cfg.Test)
}

func TestLoad_MissingTournamentName(t *testing.T) {
	path := writeTemp(t, `
data_source = "liquipedia"
page = "Y/Z"
round = "Z"
`)

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestLoad_MissingPage(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "liquipedia"
round = "Z"
`)

	_, err := Load(path)
	assert.Error(t, err)
}

func TestLoad_MissingRound(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "liquipedia"
page = "Y/Z"
`)

	_, err := Load(path)
	assert.Error(t, err)
}

func TestLoad_MissingSeriesId(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "pandascore"
round = "Z"
`)

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "series_id")
}

func TestLoad_UnsupportedDataSource(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "unknown"
round = "Z"
`)

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	assert.Error(t, err)
}

func TestLoad_InvalidTOML(t *testing.T) {
	path := writeTemp(t, "this is not = valid toml [[[")
	_, err := Load(path)
	assert.Error(t, err)
}
