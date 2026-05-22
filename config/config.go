/* config.go
 * Loads tournament configuration from a TOML file. Secrets stay in env vars.
 */

package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// Config holds the tournament configuration loaded from config.toml.
type Config struct {
	TournamentName string `toml:"tournament_name"`
	Round          string `toml:"round"`
	DataSource     string `toml:"data_source"` // liquipedia or pandascore

	// Liquipedia only fields
	Page   string `toml:"page"`   // liquipedia only
	Params string `toml:"params"` // liquipedia only, optional
	// Format overrides automatic format detection from match node sections.
	// Leave empty to auto-detect (recommended for single-stage tournaments).
	// Set explicitly (e.g. "single-elimination") when the Liquipedia page
	// contains multiple stages and auto-detection would pick the wrong one.
	Format string `toml:"format"` // liquipedia only

	// Pandascore only fields
	SeriesId int `toml:"series_id"` // PandaScore serie ID (note: API field is serie_id without 's')

	// Bot modes
	UpcomingOnly bool `toml:"upcoming_only"`
	Test         bool `toml:"test"`
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
		if c.Page == "" {
			return Config{}, fmt.Errorf("page is a required field for liquipedia as a datasource in %s", path)
		}
	case "pandascore":
		if c.SeriesId == 0 {
			return Config{}, fmt.Errorf("series_id is a required field for pandascore as a datasource in %s", path)
		}
	default:
		return Config{}, fmt.Errorf("unsupported datasource in %s, allowed values are 'liquipedia' and 'pandascore'", path)
	}

	return c, nil
}
