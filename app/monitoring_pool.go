package app

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"pickems-bot/metrics"
)

type MonitoringPool struct {
	mu      sync.RWMutex
	Entries map[int]*PoolEntry
}

type PoolEntry struct {
	DBTournamentID         int
	PandascoreTournamentID int
	SeriesId               int
	Round                  string
	KnownStatus            map[string]string
	KnownScheduleKey       string
}

// newMonitoringPool creates a new instance of MonitoringPool.
func newMonitoringPool() *MonitoringPool {
	return &MonitoringPool{
		Entries: make(map[int]*PoolEntry),
	}
}

// Subscribe adds a tournament to the monitoring pool, allowing it to be polled for updates.
// If the tournament is already in the pool, it has finished, or is a non-pandascore tournament, it does nothing.
func (a *App) Subscribe(ctx context.Context, tournamentID int) {
	a.MonitoringPool.mu.Lock()
	defer a.MonitoringPool.mu.Unlock()

	if _, exists := a.MonitoringPool.Entries[tournamentID]; exists {
		a.logger().Info("tournament already subscribed to monitoring pool, skipping", "tournament_id", tournamentID)
		return
	}

	t, err := a.Store.GetTournament(ctx, tournamentID)
	if err != nil {
		a.logger().Error("failed to subscribe tournament to monitoring pool", "error", err)
		return
	}

	if t.Source != "pandascore" {
		a.logger().Warn("cannot subscribe tournament to monitoring pool: unsupported source", "source", t.Source)
		return
	}

	if t.IsFinished {
		a.logger().Info("attempted to subscribe finished tournament to monitoring pool, skipping", "tournament_id", tournamentID)
		return
	}

	pandaScoreTournamentID, err := strconv.Atoi(t.ExternalID)
	if err != nil {
		a.logger().Error("failed to convert tournament external ID to int", "error", err)
		return
	}
	seriesID, err := strconv.Atoi(t.SeriesID)
	if err != nil {
		a.logger().Error("failed to convert tournament series ID to int", "error", err)
		return
	}

	PoolEntry := &PoolEntry{
		DBTournamentID:         t.ID,
		PandascoreTournamentID: pandaScoreTournamentID,
		SeriesId:               seriesID,
		Round:                  t.Round,
		KnownStatus:            make(map[string]string),
		KnownScheduleKey:       "",
	}
	a.MonitoringPool.Entries[tournamentID] = PoolEntry
	metrics.MonitoringPoolTournaments.WithLabelValues(
		strconv.Itoa(tournamentID), strconv.Itoa(pandaScoreTournamentID), t.Round,
	).Set(1)
}

// Unsubscribe removes a tournament from the monitoring pool, stopping it from being polled for updates.
func (a *App) Unsubscribe(ctx context.Context, tournamentID int) {
	a.MonitoringPool.mu.Lock()
	defer a.MonitoringPool.mu.Unlock()

	if entry, ok := a.MonitoringPool.Entries[tournamentID]; ok {
		metrics.MonitoringPoolTournaments.DeleteLabelValues(
			strconv.Itoa(tournamentID), strconv.Itoa(entry.PandascoreTournamentID), entry.Round,
		)
	}
	delete(a.MonitoringPool.Entries, tournamentID)
}

// BootstrapPool re-seeds the monitoring pool from every tournament currently
// referenced across guild_config. Call once at startup, before the poller
// starts ticking - the pool itself starts empty and has no memory of what was
// being tracked before a restart, so without this, any guild's tournament
// would sit unpolled until its guild re-runs /config. Subscribe already
// filters out anything that shouldn't be tracked (wrong source, already
// finished), so this can call it unconditionally for every id found.
func (a *App) BootstrapPool(ctx context.Context) error {
	ids, err := a.Store.ListTrackedTournamentIDs(ctx)
	if err != nil {
		return fmt.Errorf("BootstrapPool: %w", err)
	}
	for _, id := range ids {
		a.Subscribe(ctx, id)
	}
	return nil
}

// Snapshot returns the tournaments currently tracked by the monitoring pool.
// The returned slice is a point-in-time copy, safe to range over without
// holding the pool lock - but each *PoolEntry is the pool's live entry, not a
// copy, since the poller mutates KnownStatus/KnownScheduleKey on it directly
// as it processes a tick. That's safe only because the pool lock guards
// membership (insert/delete/lookup) and nothing else ever writes to those two
// fields on an existing entry - Subscribe only ever creates a fresh entry,
// never mutates one already in the map.
func (a *App) Snapshot() []*PoolEntry {
	a.MonitoringPool.mu.RLock()
	defer a.MonitoringPool.mu.RUnlock()

	entries := make([]*PoolEntry, 0, len(a.MonitoringPool.Entries))
	for _, entry := range a.MonitoringPool.Entries {
		entries = append(entries, entry)
	}
	return entries
}
