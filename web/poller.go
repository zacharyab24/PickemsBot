package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"pickems-bot/app"
	"pickems-bot/metrics"
	"pickems-bot/sources"
	"sort"
	"strings"
	"sync"
	"time"
)

// Poller represents the poller used for determining when to update when using PandaScore as a dataset
// since PandaScore does not support callbacks
type Poller struct {
	app      *app.App
	apiKey   string
	apiURL   string
	interval time.Duration
	log      *slog.Logger
}

// logger returns the poller's logger, falling back to the global default when none was injected.
func (p *Poller) logger() *slog.Logger {
	if p.log == nil {
		return slog.Default()
	}
	return p.log
}

// NewPoller is the poller constructor. It polls every tournament currently in
// a's monitoring pool - see app.Subscribe/app.Unsubscribe for how tournaments
// enter and leave that pool. log may be nil; if so the global slog default is
// used.
func NewPoller(a *app.App, apiKey string, apiURL string, log *slog.Logger) *Poller {
	var pollerLog *slog.Logger
	if log != nil {
		pollerLog = log.With("component", "poller")
	}
	return &Poller{
		app:      a,
		apiKey:   apiKey,
		apiURL:   apiURL,
		interval: time.Minute,
		log:      pollerLog,
	}
}

// Start runs the poller. Note this runs for the lifetime of the program.
//
// Each tournament in the pool is ticked concurrently so a slow fetch for one
// doesn't delay the others. wg.Wait() blocks the loop from going back to
// ticker.C until the whole batch finishes - ticker.C is buffered to 1 and
// drops ticks that fire while nothing is receiving, so a batch that overruns
// the interval simply skips the tick(s) it overran rather than stacking up.
func (p *Poller) Start() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for range ticker.C {
		var wg sync.WaitGroup
		for _, entry := range p.app.Snapshot() {
			wg.Add(1)
			go func(entry *app.PoolEntry) {
				defer wg.Done()
				if !p.tick(entry) {
					p.app.Unsubscribe(context.Background(), entry.DBTournamentID)
				}
			}(entry)
		}
		wg.Wait()
		p.app.RecordPoll()
	}
}

// tick is the logic that happens per tick of the poller for a single tracked
// tournament. Returns false if this tournament should be dropped from the
// monitoring pool (e.g. an unrecoverable fetch error), true otherwise.
func (p *Poller) tick(entry *app.PoolEntry) bool {
	if !p.app.Allow() {
		p.logger().Warn("rate limit reached, skipping tick")
		return true
	}

	raw, err := sources.GetPandaScoreMatches(p.apiURL, p.apiKey, entry.SeriesID, entry.PandascoreTournamentID)
	if err != nil {
		if errors.Is(err, sources.ErrUnrecoverable) {
			p.logger().Error("unrecoverable fetch error, stopping poller", "error", fmt.Errorf("poller.tick: %w", err))
			metrics.PollerErrorsTotal.Inc()
			return false
		}
		p.logger().Warn("failed to fetch matches from PandaScore, will retry next tick", "error", fmt.Errorf("poller.tick: %w", err))
		metrics.PollerErrorsTotal.Inc()
		return true
	}

	matchNodes, err := sources.ParsePandaScoreMatches(raw, entry.PandascoreTournamentID)
	if err != nil {
		p.logger().Warn("failed to parse PandaScore matches, will retry next tick", "error", fmt.Errorf("poller.tick: %w", err))
		metrics.PollerErrorsTotal.Inc()
		return true
	}

	finishedTransition := false
	for _, matchNode := range matchNodes {
		prev, seen := entry.KnownStatus[matchNode.ID]
		if seen && prev != "finished" && matchNode.Status == "finished" {
			finishedTransition = true
		}
		entry.KnownStatus[matchNode.ID] = matchNode.Status
	}

	scheduledMatches, err := sources.ParsePandaScoreSchedule(raw, entry.PandascoreTournamentID)
	if err != nil {
		p.logger().Warn("failed to parse PandaScore schedule, skipping schedule update", "error", fmt.Errorf("poller.tick: %w", err))
	} else if key := scheduleKey(scheduledMatches); key != entry.KnownScheduleKey {
		if err := p.app.StoreSchedule(context.Background(), entry.DBTournamentID, scheduledMatches); err != nil {
			p.logger().Warn("failed to store match schedule", "error", fmt.Errorf("poller.tick: %w", err))
		} else {
			p.logger().Info("match schedule updated", "matches", len(scheduledMatches))
			entry.KnownScheduleKey = key
		}
	}

	p.logger().Debug("poller tick complete", "matches_checked", len(matchNodes), "finished_transition", finishedTransition)
	metrics.PollerTicksTotal.Inc()

	if finishedTransition {
		if err := p.app.UpdateMatchResults(context.Background(), entry.DBTournamentID, entry.Round); err != nil {
			p.logger().Warn("failed to update match results", "error", fmt.Errorf("poller.tick: %w", err))
		}
	}

	return true
}

// scheduleKey returns a fingerprint of a scheduled match slice. Two slices with the
// same teams and start times (regardless of order) produce the same key, so the
// poller can detect real changes without writing to the DB every tick.
func scheduleKey(matches []sources.ScheduledMatch) string {
	type entry struct {
		team1, team2 string
		epoch        int64
	}
	entries := make([]entry, len(matches))
	for i, m := range matches {
		entries[i] = entry{m.Team1, m.Team2, m.EpochTime}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].team1 != entries[j].team1 {
			return entries[i].team1 < entries[j].team1
		}
		return entries[i].team2 < entries[j].team2
	})
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s|%s|%d;", e.team1, e.team2, e.epoch)
	}
	return b.String()
}
