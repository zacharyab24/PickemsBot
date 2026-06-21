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
	"time"
)

// Poller represents the poller used for determining when to update when using PandaScore as a dataset
// since PandaScore does not support callbacks
type Poller struct {
	app                    *app.App
	seriesID               int
	pandascoreTournamentID int // external PandaScore ID used for API filtering
	dbTournamentID         int // internal DB id used for store operations
	round                  string
	apiKey                 string
	apiURL                 string
	interval               time.Duration
	knownStatus            map[string]string // matchID -> last known status
	knownScheduleKey       string            // fingerprint of last stored schedule
	log                    *slog.Logger
}

// logger returns the poller's logger, falling back to the global default when none was injected.
func (p *Poller) logger() *slog.Logger {
	if p.log == nil {
		return slog.Default()
	}
	return p.log
}

// NewPoller is the poller constructor.
// pandascoreTournamentID is the external PandaScore ID used only for API filtering.
// dbTournamentID is the internal DB id used for all store operations.
// log may be nil; if so the global slog default is used.
func NewPoller(a *app.App, seriesID int, pandascoreTournamentID int, dbTournamentID int, round string, apiKey string, apiURL string, log *slog.Logger) *Poller {
	var pollerLog *slog.Logger
	if log != nil {
		pollerLog = log.With("component", "poller")
	}
	return &Poller{
		app:                    a,
		seriesID:               seriesID,
		pandascoreTournamentID: pandascoreTournamentID,
		dbTournamentID:         dbTournamentID,
		round:                  round,
		apiKey:                 apiKey,
		apiURL:                 apiURL,
		interval:               time.Minute,
		knownStatus:            make(map[string]string),
		log:                    pollerLog,
	}
}

// Start runs the poller. Note this runs for the lifetime of the program
func (p *Poller) Start() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for range ticker.C {
		if !p.tick() {
			return
		}
	}
}

// tick is the logic that happens per tick of the poller.
// Returns false if the poller should stop, true if it should continue.
func (p *Poller) tick() bool {
	// make sure we are not exceeding our rate limiter limitation
	if !p.app.Allow() {
		p.logger().Warn("rate limit reached, skipping tick")
		return true
	}

	raw, err := sources.GetPandaScoreMatches(p.apiURL, p.apiKey, p.seriesID, p.pandascoreTournamentID)
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

	matchNodes, err := sources.ParsePandaScoreMatches(raw, p.pandascoreTournamentID)
	if err != nil {
		p.logger().Warn("failed to parse PandaScore matches, will retry next tick", "error", fmt.Errorf("poller.tick: %w", err))
		metrics.PollerErrorsTotal.Inc()
		return true
	}

	finishedTransition := false
	for _, matchNode := range matchNodes {
		prev, seen := p.knownStatus[matchNode.ID]
		if seen && prev != "finished" && matchNode.Status == "finished" {
			finishedTransition = true
		}
		p.knownStatus[matchNode.ID] = matchNode.Status
	}

	scheduledMatches, err := sources.ParsePandaScoreSchedule(raw, p.pandascoreTournamentID)
	if err != nil {
		p.logger().Warn("failed to parse PandaScore schedule, skipping schedule update", "error", fmt.Errorf("poller.tick: %w", err))
	} else if key := scheduleKey(scheduledMatches); key != p.knownScheduleKey {
		if err := p.app.StoreSchedule(context.Background(), p.dbTournamentID, scheduledMatches); err != nil {
			p.logger().Warn("failed to store match schedule", "error", fmt.Errorf("poller.tick: %w", err))
		} else {
			p.logger().Info("match schedule updated", "matches", len(scheduledMatches))
			p.knownScheduleKey = key
		}
	}

	p.logger().Debug("poller tick complete", "matches_checked", len(matchNodes), "finished_transition", finishedTransition)
	metrics.PollerTicksTotal.Inc()

	if finishedTransition {
		if err := p.app.UpdateMatchResults(context.Background(), p.dbTournamentID, p.round); err != nil {
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
