// Package ingest holds the background jobs that keep source-derived reference
// data fresh: the tournament catalog (/config picklist) and the VRS rankings.
// Each job runs on its own schedule as a goroutine launched from main, fetching
// via sources and persisting via store.
package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"pickems-bot/app"
	"pickems-bot/sources"
	"pickems-bot/store"
)

// TournamentSync periodically refreshes the tournament catalog (upcoming +
// running PandaScore tournaments) that /config picks from, sharing the app's
// PandaScore rate limiter via app.Wait.
type TournamentSync struct {
	app      *app.App
	apiKey   string
	interval time.Duration
	log      *slog.Logger
}

// NewTournamentSync constructs a TournamentSync. interval <= 0 defaults to one
// hour; log may be nil (falls back to the slog default).
func NewTournamentSync(a *app.App, apiKey string, interval time.Duration, log *slog.Logger) *TournamentSync {
	if interval <= 0 {
		interval = time.Hour
	}
	var jobLog *slog.Logger
	if log != nil {
		jobLog = log.With("component", "ingest.tournaments")
	}
	return &TournamentSync{app: a, apiKey: apiKey, interval: interval, log: jobLog}
}

func (s *TournamentSync) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}

// Start runs an immediate sync, then repeats every interval until ctx is cancelled.
func (s *TournamentSync) Start(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger().Info("tournament sync stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce fetches the active tournament set and syncs it to the catalog. Errors
// are logged and swallowed so the job retries on the next tick.
func (s *TournamentSync) runOnce(ctx context.Context) {
	active, err := s.fetchActive(ctx)
	if err != nil {
		s.logger().Warn("tournament fetch failed, will retry next tick", "error", err)
		return
	}
	if len(active) == 0 {
		// An empty set almost always means a bad API response; syncing it would
		// wrongly mark every catalog row finished, so skip rather than sweep.
		s.logger().Warn("no active tournaments returned, skipping sync")
		return
	}
	newlyFinished, err := s.app.Store.SyncTournaments(ctx, active)
	if err != nil {
		s.logger().Warn("tournament sync failed", "error", err)
		return
	}
	// A tournament that just finished is done for good - unsubscribe it from
	// the monitoring pool regardless of how many guilds still have it configured.
	// finalizeFinishedTournament runs one last results fetch first: the
	// catalog-level "finished" signal (this hourly sync) can arrive before the
	// poller's own per-minute tick notices the last match completed, and once
	// unsubscribed nothing else will ever fetch that match's result again.
	for _, id := range newlyFinished {
		if err := s.finalizeFinishedTournament(ctx, id); err != nil {
			s.logger().Warn("failed to finalize finished tournament before unsubscribing", "tournament_id", id, "error", err)
		}
		s.app.Unsubscribe(ctx, id)
	}

	// Re-subscribe anything guild_config still tracks but that's missing from
	// the pool - e.g. a tournament the poller dropped after an unrecoverable
	// fetch error (see Poller.tick). No dedicated reconciliation loop exists
	// elsewhere, so this hourly sync doubles as one; Subscribe no-ops for
	// anything already tracked, so this is cheap.
	if err := s.app.ReconcilePool(ctx); err != nil {
		s.logger().Warn("pool reconciliation failed", "error", err)
	}

	s.logger().Info("tournament catalog synced", "count", len(active), "newly_finished", len(newlyFinished))
}

// finalizeFinishedTournament fetches id's own round and does one last
// rate-limited results fetch, so a match that finished right before the
// catalog marked the tournament done isn't lost when it's unsubscribed.
func (s *TournamentSync) finalizeFinishedTournament(ctx context.Context, id int) error {
	t, err := s.app.Store.GetTournament(ctx, id)
	if err != nil {
		return fmt.Errorf("get tournament: %w", err)
	}
	return s.app.UpdateMatchResults(ctx, t.ID, t.Round, t.Source)
}

// fetchActive returns the combined upcoming + running tournament set, each fetch
// gated on the shared rate limiter. Both fetches must succeed: a partial set
// would make live tournaments look finished during the catalog sweep.
func (s *TournamentSync) fetchActive(ctx context.Context) ([]store.TournamentCatalogEntry, error) {
	if err := s.app.Wait(ctx, "pandascore"); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}
	upcoming, err := sources.GetUpcomingPandaScoreTournaments(s.apiKey)
	if err != nil {
		return nil, fmt.Errorf("fetch upcoming: %w", err)
	}

	if err := s.app.Wait(ctx, "pandascore"); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}
	running, err := sources.GetRunningPandaScoreTournaments(s.apiKey)
	if err != nil {
		return nil, fmt.Errorf("fetch running: %w", err)
	}

	all := append(upcoming, running...)
	entries := make([]store.TournamentCatalogEntry, 0, len(all))
	for _, t := range all {
		entries = append(entries, store.TournamentCatalogEntry{
			ExternalID: strconv.Itoa(t.ID),
			Name:       t.DisplayName(),
			Round:      t.Round,
			SeriesID:   strconv.Itoa(t.SerieID),
		})
	}
	return entries, nil
}
