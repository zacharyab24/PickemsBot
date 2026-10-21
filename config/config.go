/* config.go
 * Loads deployment configuration from a TOML file. Secrets stay in env vars.
 * Tournaments are chosen per channel with /config; the tournament fields here
 * only bootstrap the Liquipedia webhook server.
 */

package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds the deployment configuration loaded from config.toml.
type Config struct {
	DataSource string `toml:"data_source"`  // liquipedia or pandascore
	GuildID    string `toml:"dev_guild_id"` // optional; if set, bot will only register commands in this guild (faster for testing)

	// Test selects the beta Discord token and defaults logging to debug.
	Test bool `toml:"test"`

	// LogLevel controls the minimum log level emitted (debug, info, warn, error).
	// Defaults to "info". When test=true and log_level is unset, defaults to "debug".
	LogLevel string `toml:"log_level"`

	// Liquipedia webhook only: the tournament the webhook server updates.
	TournamentName string `toml:"tournament_name"`
	Round          string `toml:"round"`

	Liquipedia LiquipediaConfig `toml:"liquipedia"`
	PandaScore PandaScoreConfig `toml:"pandascore"`
}

// LiquipediaConfig holds Liquipedia-specific configuration fields.
type LiquipediaConfig struct {
	APIURL string `toml:"api_url"`
	Page   string `toml:"page"`
}

// PandaScoreConfig holds PandaScore-specific configuration fields.
type PandaScoreConfig struct {
	APIURL string `toml:"api_url"`
}

// Load reads and validates a config.toml file at path.
func Load(path string) (Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}

	switch c.DataSource {
	case "liquipedia":
		if c.TournamentName == "" || c.Round == "" || c.Liquipedia.Page == "" {
			return Config{}, fmt.Errorf("tournament_name, round and liquipedia.page are required for liquipedia as a datasource in %s", path)
		}
		if strings.TrimSpace(c.Liquipedia.APIURL) == "" {
			return Config{}, fmt.Errorf("liquipedia.api_url is a required field for liquipedia as a datasource in %s", path)
		}
	case "pandascore":
		if strings.TrimSpace(c.PandaScore.APIURL) == "" {
			return Config{}, fmt.Errorf("pandascore.api_url is a required field for pandascore as a datasource in %s", path)
		}
	default:
		return Config{}, fmt.Errorf("unsupported datasource in %s, allowed values are 'liquipedia' and 'pandascore'", path)
	}

	return c, nil
}
