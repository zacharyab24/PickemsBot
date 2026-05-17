package web

import (
	"encoding/json"
	"log"
	"net/http"
	"pickems-bot/api/external"
	"strings"

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
		log.Println("failed to decode webhook:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if event.Wiki != "counterstrike" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !isRelevantTournamentPage(event.Page, s.api.Store.GetPage()) {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("Recieved relevant webhook, running update functions")

	// Kick async pipeline – call into your existing packages (/api, /bot, etc)
	go func(e LiquipediaEvent) {
		if err := s.api.UpdateMatchResults(); err != nil {
			log.Println("RefreshTournamentData failed:", err)
			return
		}
		if err := s.api.GenerateLeaderboard(); err != nil {
			log.Println("RecalculateAndStoreLeaderboard failed:", err)
			return
		}
		nodes, kind, err := s.api.Store.FetchMatchNodesFromDb()
		if err != nil {
			log.Println("FetchMatchNodesFromDb failed: ", err)
			return
		}
		if kind == "" {
			log.Println("kind was empty, cannot generate results render")
			return
		}
		renderNodes := toRenderNodes(nodes)
		if err := render.RenderBracket(renderNodes, string(kind), s.api.Store.GetRound(), "resources/result.png"); err != nil {
			log.Println("RenderBracket failed: ", err)
		}
	}(event)

	w.WriteHeader(http.StatusOK)
}

// toRenderNodes converts external.MatchNode slice to the renderer's input type.
// The two types are structurally identical; this bridges the package boundary.
func toRenderNodes(nodes []external.MatchNode) []render.MatchNode {
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
