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
	Page           string `toml:"page"`
	Round          string `toml:"round"`
	Params         string `toml:"params"`
	UpcomingOnly   bool   `toml:"upcoming_only"`
	Test           bool   `toml:"test"`
}

// Load reads and validates a config.toml file at path.
func Load(path string) (Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if c.TournamentName == "" || c.Page == "" || c.Round == "" {
		return Config{}, fmt.Errorf("tournament_name, page, and round are required in %s", path)
	}
	return c, nil
}
