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
	// Format overrides automatic format detection from match node sections.
	// Leave empty to auto-detect (recommended for single-stage tournaments).
	// Set explicitly (e.g. "single-elimination") when the Liquipedia page
	// contains multiple stages and auto-detection would pick the wrong one.
	Format       string `toml:"format"`
	UpcomingOnly bool   `toml:"upcoming_only"`
	Test         bool   `toml:"test"`
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
