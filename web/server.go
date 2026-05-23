//go:build !test

/* server.go
 * Contains the HTTP server Start function that listens for incoming connections.
 * Excluded from test coverage as it blocks and requires real network binding.
 * Author: Zachary Bower
 */

package web

import (
	"log"
	"net/http"
	"pickems-bot/app"
	"time"
)

// Config holds the configuration for the web server
type Config struct {
	Addr string
	API  *app.App
}

// Server is the HTTP server that handles webhook requests
type Server struct {
	api *app.App
}

// Start initializes and starts the HTTP server with the given configuration
func Start(cfg Config) error {
	s := &Server{
		api: cfg.API,
	}

	mux := http.NewServeMux()
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
