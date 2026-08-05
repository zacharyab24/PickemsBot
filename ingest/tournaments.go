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
// running PandaScore tournaments) that /config picks from. It shares the app's
// PandaScore rate limiter via app.Wait, so its calls and the live-match poller's
// draw from one token bucket and together stay within the API limit.
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
	// A tournament that just finished is done for good - drop it from the
	// poller's monitoring pool regardless of how many guilds still have it
	// configured, rather than keep burning API calls on a decided bracket.
	for _, id := range newlyFinished {
		s.app.Unsubscribe(ctx, id)
	}
	s.logger().Info("tournament catalog synced", "count", len(active), "newly_finished", len(newlyFinished))
}

// fetchActive returns the combined upcoming + running tournament set, each fetch
// gated on the shared rate limiter. Both fetches must succeed: a partial set
// would make live tournaments look finished during the catalog sweep.
func (s *TournamentSync) fetchActive(ctx context.Context) ([]store.TournamentCatalogEntry, error) {
	if err := s.app.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}
	upcoming, err := sources.GetUpcomingPandaScoreTournaments(s.apiKey)
	if err != nil {
		return nil, fmt.Errorf("fetch upcoming: %w", err)
	}

	if err := s.app.Wait(ctx); err != nil {
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
