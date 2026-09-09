package app

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"pickems-bot/metrics"
)

// MonitoringPool is the in-memory set of tournaments the poller is tracking.
// mu guards Entries only - it does not guard the fields inside a *PoolEntry.
type MonitoringPool struct {
	mu sync.RWMutex

	Entries    map[int]*PoolEntry
	lastPollAt time.Time
}

// PoolEntry is one tournament the poller is tracking. Only the poller
// mutates KnownStatus/KnownScheduleKey, so that's safe without locking.
type PoolEntry struct {
	DBTournamentID         int
	Name                   string
	PandascoreTournamentID int
	SeriesID               int
	Round                  string
	KnownStatus            map[string]string
	KnownScheduleKey       string
}

// newMonitoringPool creates a new MonitoringPool.
func newMonitoringPool() *MonitoringPool {
	return &MonitoringPool{
		Entries: make(map[int]*PoolEntry),
	}
}

// Subscribe adds a tournament to the monitoring pool. No-op if it's already
// tracked, finished, or not a pandascore tournament.
//
// The write lock is only held for the map check and the final insert - not
// across the GetTournament call in between, so a slow/stalled DB here doesn't
// stall the poller's tick loop or the /poller telemetry endpoint (both need
// the same lock via Snapshot()). Two concurrent Subscribe calls for the same
// tournamentID can both reach GetTournament; the second one to acquire the
// lock afterwards just finds the entry already there and backs off.
func (a *App) Subscribe(ctx context.Context, tournamentID int) {
	a.MonitoringPool.mu.RLock()
	_, exists := a.MonitoringPool.Entries[tournamentID]
	a.MonitoringPool.mu.RUnlock()
	if exists {
		a.logger().Info("tournament already subscribed to monitoring pool, skipping", "tournament_id", tournamentID)
		return
	}

	t, err := a.Store.GetTournament(ctx, tournamentID)
	if err != nil {
		a.logger().Error("failed to subscribe tournament to monitoring pool", "error", err)
		return
	}

	if t.Source != sourcePandaScore {
		a.logger().Warn("cannot subscribe tournament to monitoring pool: unsupported source", "source", t.Source)
		return
	}

	if t.IsFinished {
		a.logger().Info("attempted to subscribe finished tournament to monitoring pool, skipping", "tournament_id", tournamentID)
		return
	}

	pandaScoreTournamentID, seriesID, err := parsePandaScoreIDs(t)
	if err != nil {
		a.logger().Error("failed to subscribe tournament to monitoring pool", "error", err)
		return
	}

	entry := &PoolEntry{
		DBTournamentID:         t.ID,
		Name:                   t.Name,
		PandascoreTournamentID: pandaScoreTournamentID,
		SeriesID:               seriesID,
		Round:                  t.Round,
		KnownStatus:            make(map[string]string),
		KnownScheduleKey:       "",
	}

	a.MonitoringPool.mu.Lock()
	if _, exists := a.MonitoringPool.Entries[tournamentID]; exists {
		a.MonitoringPool.mu.Unlock()
		a.logger().Info("tournament already subscribed to monitoring pool, skipping", "tournament_id", tournamentID)
		return
	}
	a.MonitoringPool.Entries[tournamentID] = entry
	a.MonitoringPool.mu.Unlock()

	metrics.MonitoringPoolTournaments.WithLabelValues(
		strconv.Itoa(tournamentID), strconv.Itoa(pandaScoreTournamentID), t.Round,
	).Set(1)
}

// Unsubscribe removes a tournament from the monitoring pool.
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

// BootstrapPool re-seeds the monitoring pool from every tournament tracked in
// guild_config. Subscribe alone seeds a fresh, empty KnownStatus map, so a
// match that finished while the bot was down (deploy/crash) would never fire
// a finishedTransition on the poller's next tick - PopulateMatches is called
// directly here for each freshly-subscribed entry to backfill those results
// before the poller starts ticking.
func (a *App) BootstrapPool(ctx context.Context) error {
	ids, err := a.Store.ListTrackedTournamentIDs(ctx)
	if err != nil {
		return fmt.Errorf("BootstrapPool: %w", err)
	}
	for _, id := range ids {
		a.Subscribe(ctx, id)

		a.MonitoringPool.mu.RLock()
		entry, tracked := a.MonitoringPool.Entries[id]
		a.MonitoringPool.mu.RUnlock()
		if !tracked {
			continue
		}

		if err := a.PopulateMatches(ctx, entry.DBTournamentID, entry.Round, sourcePandaScore, false); err != nil {
			a.logger().Warn("BootstrapPool: backfill populate failed", "tournament_id", id, "error", err)
		}
	}
	return nil
}

// ReconcilePool re-subscribes any tournament tracked in guild_config that's
// missing from the monitoring pool - e.g. one the poller dropped after an
// unrecoverable fetch error (see Poller.tick). Already-subscribed tournaments
// are left untouched: Subscribe no-ops for them, so this is cheap to call
// repeatedly. Unlike BootstrapPool, no backfill populate runs here - a
// tournament only reaches this state after having been ticked at least once.
func (a *App) ReconcilePool(ctx context.Context) error {
	ids, err := a.Store.ListTrackedTournamentIDs(ctx)
	if err != nil {
		return fmt.Errorf("ReconcilePool: %w", err)
	}
	for _, id := range ids {
		a.Subscribe(ctx, id)
	}
	return nil
}

// RecordPoll marks that the poller just completed a full tick cycle.
func (a *App) RecordPoll() {
	a.MonitoringPool.mu.Lock()
	defer a.MonitoringPool.mu.Unlock()
	a.MonitoringPool.lastPollAt = time.Now()
}

// LastPollTime returns when the poller last completed a tick cycle, and false if it hasn't yet.
func (a *App) LastPollTime() (time.Time, bool) {
	a.MonitoringPool.mu.RLock()
	defer a.MonitoringPool.mu.RUnlock()
	if a.MonitoringPool.lastPollAt.IsZero() {
		return time.Time{}, false
	}
	return a.MonitoringPool.lastPollAt, true
}

// Snapshot returns the tournaments currently tracked. Safe to range over
// without the lock - each *PoolEntry is the pool's live entry, not a copy.
func (a *App) Snapshot() []*PoolEntry {
	a.MonitoringPool.mu.RLock()
	defer a.MonitoringPool.mu.RUnlock()

	entries := make([]*PoolEntry, 0, len(a.MonitoringPool.Entries))
	for _, entry := range a.MonitoringPool.Entries {
		entries = append(entries, entry)
	}
	return entries
}
