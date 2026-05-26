package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"pickems-bot/app"
	"pickems-bot/metrics"
	"pickems-bot/sources"
	"pickems-bot/tournament"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zacharyab24/pickems-renderer/render"
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
		if err := s.api.UpdateMatchResults(); err != nil {
			s.logger().Error("update match results failed", "error", fmt.Errorf("webhook pipeline: %w", err))
			return
		}
		if err := s.api.GenerateLeaderboard(); err != nil {
			s.logger().Error("generate leaderboard failed", "error", fmt.Errorf("webhook pipeline: %w", err))
			return
		}
		if err := RenderResultsImage(s.api); err != nil {
			s.logger().Error("render results image failed", "error", fmt.Errorf("webhook pipeline: %w", err))
		}
	}(event)

	w.WriteHeader(http.StatusOK)
}

const resultImagePath = "resources/result.png"

// RenderResultsImage fetches match nodes from the DB and regenerates the result image on disk.
// It is called at startup and after each webhook update to ensure the image is always current.
func RenderResultsImage(a *app.App) error {
	timer := prometheus.NewTimer(metrics.ImageRenderDuration)
	defer timer.ObserveDuration()

	if err := os.MkdirAll("resources", 0755); err != nil {
		return fmt.Errorf("failed to create resources directory: %w", err)
	}
	nodes, kind, err := a.Store.FetchMatchNodesFromDb()
	if err != nil {
		return fmt.Errorf("failed to fetch match nodes: %w", err)
	}
	if kind == "" {
		return fmt.Errorf("kind was empty, cannot generate results image")
	}
	// Trim 3rd-place consolation match and normalise section names so the
	// renderer places each match in the correct column. Liquipedia returns
	// all bracket nodes with Section = "Bracket/8" (the template name), but
	// the renderer groups by Section to build columns and only recognises
	// names like "Quarterfinals", "Semifinals", "Grand Final".
	if kind == tournament.SingleElim {
		nodes = tournament.TrimSingleElimNodes(nodes)
		nodes = tournament.NormalizeSingleElimSections(nodes)
	}

	renderNodes := toRenderNodes(nodes)
	if err := render.RenderBracket(renderNodes, string(kind), a.Store.GetRound(), resultImagePath); err != nil {
		return fmt.Errorf("RenderBracket failed: %w", err)
	}
	return nil
}

// toRenderNodes converts sources.MatchNode slice to the renderer's input type.
// The two types are structurally identical; this bridges the package boundary.
func toRenderNodes(nodes []sources.MatchNode) []render.MatchNode {
	out := make([]render.MatchNode, len(nodes))
	for i, n := range nodes {
		out[i] = render.MatchNode{
			ID:      n.ID,
			Team1:   n.Team1,
			Team2:   n.Team2,
			Winner:  n.Winner,
			Score:   n.Score,
			Section: n.Section,
		}
	}
	return out
}
