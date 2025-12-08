package web

import (
	"log"
	"net/http"
	"pickems-bot/api/api"
	"time"
)

// Config holds the configuration for the web server
type Config struct {
	Addr string
	API  *api.API
}

// Server is the HTTP server that handles webhook requests
type Server struct {
	api *api.API
}

// Start initializes and starts the HTTP server with the given configuration
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
