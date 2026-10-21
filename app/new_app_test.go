package app

import (
	"testing"

	"pickems-bot/config"
)

func TestNewApp_RejectsUnsupportedDataSource(t *testing.T) {
	if _, err := NewApp(config.Config{DataSource: "hltv"}, "postgres://u@localhost/db", nil); err == nil {
		t.Fatal("expected an error for an unknown data source")
	}
}

func TestNewApp_RequiresPostgresURI(t *testing.T) {
	if _, err := NewApp(config.Config{DataSource: "pandascore"}, "", nil); err == nil {
		t.Fatal("expected an error for an empty postgres URI")
	}
}

// Both limiters exist regardless of the configured source, since guild_config
// can point at a tournament from either one.
func TestNewApp_BuildsLimitersForBothSources(t *testing.T) {
	a, err := NewApp(config.Config{DataSource: "liquipedia"}, "postgres://u@localhost/db", nil)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(a.Store.Close)

	for _, source := range []string{"pandascore", "liquipedia"} {
		if !a.Allow(source) {
			t.Errorf("expected a fresh %s limiter to allow a request", source)
		}
	}
	if len(a.Snapshot()) != 0 {
		t.Error("expected an empty monitoring pool")
	}
}
