package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// LiquipediaEvent represents the webhook event payload from Liquipedia
type LiquipediaEvent struct {
	Wiki  string `json:"wiki"`
	Page  string `json:"page"`
	Event string `json:"event"`
}

func isRelevantTournamentPage(page, base string) bool {
	if page == base {
		return true
	}
	return strings.HasPrefix(page, base+"/")
}

// LiquipediaWebhookHandler HTTP endpoint that receives a webhook from the LiquipediaDB api used to kick off
// updating stored data and calculate user scores
// Preconditions: HTTP server has been started, receives HTTP ResponseWriter and Http Request
// Postconditions: Kicks off the update functions for the MatchResults data and Leaderboard data
func (s *Server) LiquipediaWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var event LiquipediaEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		s.logger().Error("failed to decode webhook body", "error", fmt.Errorf("LiquipediaWebhookHandler: %w", err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if event.Wiki != "counterstrike" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !isRelevantTournamentPage(event.Page, s.page) {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.logger().Info("received relevant webhook, running update pipeline", "page", event.Page)

	// Kick async pipeline – call into your existing packages (/api, /bot, etc)
	go func(e LiquipediaEvent) {
		ctx := context.Background()
		if err := s.api.UpdateMatchSchedule(ctx, s.tournamentID); err != nil {
			s.logger().Warn("update match schedule failed", "error", fmt.Errorf("webhook pipeline: %w", err))
		}
		if err := s.api.UpdateMatchResults(ctx, s.tournamentID, s.round); err != nil {
			s.logger().Error("update match results failed", "error", fmt.Errorf("webhook pipeline: %w", err))
		}
	}(event)

	w.WriteHeader(http.StatusOK)
}
