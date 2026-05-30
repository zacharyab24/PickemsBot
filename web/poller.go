package web

import (
	"errors"
	"fmt"
	"log/slog"
	"pickems-bot/app"
	"pickems-bot/metrics"
	"pickems-bot/sources"
	"time"
)

// Poller represents the poller used for determining when to update when using PandaScore as a dataset
// since PandaScore does not support callbacks
type Poller struct {
	app          *app.App
	seriesID     int
	tournamentID int
	apiKey       string
	apiURL       string
	interval     time.Duration
	knownStatus  map[string]string // matchID -> last known status
	log          *slog.Logger
}

// logger returns the poller's logger, falling back to the global default when none was injected.
func (p *Poller) logger() *slog.Logger {
	if p.log == nil {
		return slog.Default()
	}
	return p.log
}

// NewPoller is the poller constructor.
// log may be nil; if so the global slog default is used.
func NewPoller(a *app.App, seriesID int, tournamentID int, apiKey string, apiURL string, log *slog.Logger) *Poller {
	var pollerLog *slog.Logger
	if log != nil {
		pollerLog = log.With("component", "poller")
	}
	return &Poller{
		app:          a,
		seriesID:     seriesID,
		tournamentID: tournamentID,
		apiKey:       apiKey,
		apiURL:       apiURL,
		interval:     time.Minute,
		knownStatus:  make(map[string]string),
		log:          pollerLog,
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

	raw, err := sources.GetPandaScoreMatches(p.apiURL, p.apiKey, p.seriesID, p.tournamentID)
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

	matchNodes, err := sources.ParsePandaScoreMatches(raw, p.tournamentID)
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

	p.logger().Debug("poller tick complete", "matches_checked", len(matchNodes), "finished_transition", finishedTransition)
	metrics.PollerTicksTotal.Inc()

	if finishedTransition {
		if err := p.app.UpdateMatchResults(); err != nil {
			p.logger().Warn("failed to update match results", "error", fmt.Errorf("poller.tick: %w", err))
		}
		if err := p.app.GenerateLeaderboard(); err != nil {
			p.logger().Warn("failed to generate leaderboard", "error", fmt.Errorf("poller.tick: %w", err))
		}
		if err := RenderResultsImage(p.app); err != nil {
			p.logger().Warn("failed to render results image", "error", fmt.Errorf("poller.tick: %w", err))
		}
	}

	return true
}
