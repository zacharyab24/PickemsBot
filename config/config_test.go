package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTemp(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
	return path
}

func TestLoad_Liquipedia(t *testing.T) {
	path := writeTemp(t, `
data_source = "liquipedia"
tournament_name = "MyEvent_2026"
round = "Stage_1"
test = true
log_level = "warn"
dev_guild_id = "123"

[liquipedia]
api_url = "https://api.liquipedia.net/api/v3/match"
page = "Foo/2026/Bar/Stage_1"
`)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, Config{
		DataSource:     "liquipedia",
		GuildID:        "123",
		Test:           true,
		LogLevel:       "warn",
		TournamentName: "MyEvent_2026",
		Round:          "Stage_1",
		Liquipedia:     LiquipediaConfig{APIURL: "https://api.liquipedia.net/api/v3/match", Page: "Foo/2026/Bar/Stage_1"},
	}, cfg)
}

// PandaScore tournaments come from /config, so only the API URL is needed.
func TestLoad_PandaScore_OnlyNeedsAPIURL(t *testing.T) {
	path := writeTemp(t, `
data_source = "pandascore"

[pandascore]
api_url = "https://api.pandascore.co/csgo/matches"
`)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "pandascore", cfg.DataSource)
	assert.Equal(t, "https://api.pandascore.co/csgo/matches", cfg.PandaScore.APIURL)
	assert.False(t, cfg.Test)
}

// Configs written before the field audit must still load.
func TestLoad_IgnoresRetiredFields(t *testing.T) {
	path := writeTemp(t, `
data_source = "pandascore"
tournament_name = "Old"
round = "Playoffs"
upcoming_only = false

[liquipedia]
params = ""
format = ""

[pandascore]
api_url = "https://api.pandascore.co/csgo/matches"
series_id = 10488
tournament_id = 20710
`)

	_, err := Load(path)
	assert.NoError(t, err)
}

func TestLoad_Invalid(t *testing.T) {
	tests := map[string]struct {
		contents  string
		wantInErr string
	}{
		"liquipedia missing tournament_name": {`
data_source = "liquipedia"
round = "Z"
[liquipedia]
api_url = "u"
page = "Y/Z"
`, "tournament_name"},
		"liquipedia missing round": {`
data_source = "liquipedia"
tournament_name = "X"
[liquipedia]
api_url = "u"
page = "Y/Z"
`, "round"},
		"liquipedia missing page": {`
data_source = "liquipedia"
tournament_name = "X"
round = "Z"
[liquipedia]
api_url = "u"
`, "liquipedia.page"},
		"liquipedia missing api_url": {`
data_source = "liquipedia"
tournament_name = "X"
round = "Z"
[liquipedia]
page = "Y/Z"
`, "liquipedia.api_url"},
		"pandascore blank api_url": {`
data_source = "pandascore"
[pandascore]
api_url = "  "
`, "pandascore.api_url"},
		"unsupported data source": {`data_source = "hltv"`, "unsupported"},
		"missing data source":     {`test = true`, "unsupported"},
		"invalid toml":            {`this is not = valid toml [[[`, "decode"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(writeTemp(t, tt.contents))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantInErr)
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	assert.Error(t, err)
}
