/* config.go
 * Loads tournament configuration from a TOML file. Secrets stay in env vars.
 */

package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds the tournament configuration loaded from config.toml.
type Config struct {
	TournamentName string `toml:"tournament_name"`
	Round          string `toml:"round"`
	DataSource     string `toml:"data_source"`  // liquipedia or pandascore
	GuildID        string `toml:"dev_guild_id"` // optional; if set, bot will only register commands in this guild (faster for testing)

	// Bot modes
	UpcomingOnly bool `toml:"upcoming_only"`
	Test         bool `toml:"test"`

	// Logging
	// LogLevel controls the minimum log level emitted (debug, info, warn, error).
	// Defaults to "info". When test=true and log_level is unset, defaults to "debug".
	LogLevel string `toml:"log_level"`

	Liquipedia LiquipediaConfig `toml:"liquipedia"`
	PandaScore PandaScoreConfig `toml:"pandascore"`
}

// LiquipediaConfig holds Liquipedia-specific configuration fields.
type LiquipediaConfig struct {
	APIURL string `toml:"api_url"`
	Page   string `toml:"page"`
	Params string `toml:"params"`
	// Format overrides automatic format detection from match node sections.
	// Leave empty to auto-detect (recommended for single-stage tournaments).
	// Set explicitly (e.g. "single-elimination") when the Liquipedia page
	// contains multiple stages and auto-detection would pick the wrong one.
	Format string `toml:"format"`
}

// PandaScoreConfig holds PandaScore-specific configuration fields.
type PandaScoreConfig struct {
	APIURL       string `toml:"api_url"`
	SeriesID     int    `toml:"series_id"`     // filters by serie_id; covers all stages in a major
	TournamentID int    `toml:"tournament_id"` // optional; narrows to a single stage within the series
}

// Load reads and validates a config.toml file at path.
func Load(path string) (Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}

	if c.DataSource == "" || c.TournamentName == "" || c.Round == "" {
		return Config{}, fmt.Errorf("data_source, tournament_name and round are required in %s", path)
	}

	switch c.DataSource {
	case "liquipedia":
		if c.Liquipedia.Page == "" {
			return Config{}, fmt.Errorf("liquipedia.page is a required field for liquipedia as a datasource in %s", path)
		}
		if strings.TrimSpace(c.Liquipedia.APIURL) == "" {
			return Config{}, fmt.Errorf("liquipedia.api_url is a required field for liquipedia as a datasource in %s", path)
		}
	case "pandascore":
		if c.PandaScore.SeriesID == 0 && c.PandaScore.TournamentID == 0 {
			return Config{}, fmt.Errorf("pandascore.series_id or pandascore.tournament_id is required for pandascore as a datasource in %s", path)
		}
		if strings.TrimSpace(c.PandaScore.APIURL) == "" {
			return Config{}, fmt.Errorf("pandascore.api_url is a required field for pandascore as a datasource in %s", path)
		}
	default:
		return Config{}, fmt.Errorf("unsupported datasource in %s, allowed values are 'liquipedia' and 'pandascore'", path)
	}

	return c, nil
}
