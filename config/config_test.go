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
round = "Stage_1"
upcoming_only = true
test = true

[liquipedia]
api_url = "https://api.liquipedia.net/api/v3/match"
page = "Foo/2026/Bar/Stage_1"
`)

	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.Equal(t, "MyEvent_2026", cfg.TournamentName)
	assert.Equal(t, "liquipedia", cfg.DataSource)
	assert.Equal(t, "https://api.liquipedia.net/api/v3/match", cfg.Liquipedia.APIURL)
	assert.Equal(t, "Foo/2026/Bar/Stage_1", cfg.Liquipedia.Page)
	assert.Equal(t, "Stage_1", cfg.Round)
	assert.True(t, cfg.UpcomingOnly)
	assert.True(t, cfg.Test)
}

func TestLoad_AllFields_PandaScore(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "MyEvent_2026"
data_source = "pandascore"
round = "Stage_1"
upcoming_only = true
test = true

[pandascore]
api_url = "https://api.pandascore.co/csgo/matches"
series_id = 10583
`)

	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.Equal(t, "MyEvent_2026", cfg.TournamentName)
	assert.Equal(t, "pandascore", cfg.DataSource)
	assert.Equal(t, "https://api.pandascore.co/csgo/matches", cfg.PandaScore.APIURL)
	assert.Equal(t, 10583, cfg.PandaScore.SeriesID)
	assert.Equal(t, "Stage_1", cfg.Round)
	assert.True(t, cfg.UpcomingOnly)
	assert.True(t, cfg.Test)
}

func TestLoad_DefaultBoolsAndOptionalParams(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "liquipedia"
round = "Z"

[liquipedia]
api_url = "https://api.liquipedia.net/api/v3/match"
page = "Y/Z"
`)

	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.False(t, cfg.UpcomingOnly)
	assert.False(t, cfg.Test)
}

func TestLoad_MissingTournamentName(t *testing.T) {
	path := writeTemp(t, `
data_source = "liquipedia"
round = "Z"

[liquipedia]
api_url = "https://api.liquipedia.net/api/v3/match"
page = "Y/Z"
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

[liquipedia]
api_url = "https://api.liquipedia.net/api/v3/match"
`)

	_, err := Load(path)
	assert.Error(t, err)
}

func TestLoad_MissingAPIURL_Liquipedia(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "liquipedia"
round = "Z"

[liquipedia]
page = "Y/Z"
`)

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_url")
}

func TestLoad_MissingRound(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "liquipedia"

[liquipedia]
api_url = "https://api.liquipedia.net/api/v3/match"
page = "Y/Z"
`)

	_, err := Load(path)
	assert.Error(t, err)
}

func TestLoad_MissingSeriesID(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "pandascore"
round = "Z"

[pandascore]
api_url = "https://api.pandascore.co/csgo/matches"
`)

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "series_id")
}

func TestLoad_MissingAPIURL_PandaScore(t *testing.T) {
	path := writeTemp(t, `
tournament_name = "X"
data_source = "pandascore"
round = "Z"

[pandascore]
series_id = 12345
`)

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_url")
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
