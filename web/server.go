package web

import (
	"log"
	"net/http"
	"pickems-bot/api/api"
	"time"
)

type Config struct {
	Addr string
	API  *api.API
}

type Server struct {
	api *api.API
}

func Start(cfg Config) error {
	s := &Server{
		api: cfg.API,
	}

	mux := http.NewServeMux()
	// bind handler methods that have access to s.api
	mux.HandleFunc("/webhooks/liquipedia", s.LiquipediaWebhookHandler)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	log.Println("HTTP server listening on", cfg.Addr)
	return srv.ListenAndServe()
}
