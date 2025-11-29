package web

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

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

	expectedWiki := "counterstrike"
	basePage := os.Getenv("PAGE")

	if event.Wiki != expectedWiki {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !isRelevantTournamentPage(event.Page, basePage) {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("Liquipedia event wiki=%s page=%s event=%s\n", event.Wiki, event.Page, event.Event)

	// Kick async pipeline – call into your existing packages (/api, /bot, etc)
	go func(e LiquipediaEvent) {
		if err := s.api.Store.FetchAndUpdateMatchResults(); err != nil {
			log.Println("RefreshTournamentData failed:", err)
			return
		}
		// Need to add leaderboard storage and refactor calc function
		//if err := RecalculateAndStoreLeaderboard(); err != nil {
		//	log.Println("RecalculateAndStoreLeaderboard failed:", err)
		//	return
		//}
	}(event)

	w.WriteHeader(http.StatusOK)
}
