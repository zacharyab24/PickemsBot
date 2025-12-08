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
	"time"
)

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
