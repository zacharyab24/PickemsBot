package web

import (
	"errors"
	"log"
	"pickems-bot/app"
	"pickems-bot/sources"
	"time"
)

// Poller represents the poller used for determining when to update when using PandaScore as a dataset
// since PandaScore does not support callbacks
type Poller struct {
	app         *app.App
	seriesId    int
	apiKey      string
	interval    time.Duration
	knownStatus map[string]string // matchID -> last known status
}

// NewPoller is the poller constructor
func NewPoller(a *app.App, seriesId int, apiKey string) *Poller {
	return &Poller{
		app:         a,
		seriesId:    seriesId,
		apiKey:      apiKey,
		interval:    time.Minute,
		knownStatus: make(map[string]string),
	}
}

// Start runs the poller. Note this runs for the lifetime of the program
func (p *Poller) Start() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for range ticker.C {
		if !p.tick() {
			log.Println("Poller: stopping due to unrecoverable error")
			return
		}
	}
}

// tick is the logic that happens per tick of the poller.
// Returns false if the poller should stop, true if it should continue.
func (p *Poller) tick() bool {
	// make sure we are not exceeding our rate limiter limitation
	if !p.app.Allow() {
		log.Println("Poller: rate limit reached, skipping tick")
		return true
	}

	raw, err := sources.GetPandaScoreMatches(p.apiKey, p.seriesId)
	if err != nil {
		log.Printf("Poller: fetch error: %v", err)
		return !errors.Is(err, sources.ErrUnrecoverable)
	}

	matchNodes, err := sources.ParsePandaScoreMatches(raw)
	if err != nil {
		log.Printf("Poller: parse error: %v", err)
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

	if finishedTransition {
		if err := p.app.UpdateMatchResults(); err != nil {
			log.Printf("Poller: update match results error: %v", err)
		}
		if err := p.app.GenerateLeaderboard(); err != nil {
			log.Printf("Poller: generate leaderboard error: %v", err)
		}
		if err := RenderResultsImage(p.app); err != nil {
			log.Printf("Poller: render results image error: %v", err)
		}
	}

	return true
}
