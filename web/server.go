/* server.go
 * Contains the HTTP server Start function that listens for incoming connections.
 * Excluded from test coverage as it blocks and requires real network binding.
 * Author: Zachary Bower
 */

package web

import (
	"log/slog"
	"net/http"
	"pickems-bot/app"
	"time"
)

// Config holds the configuration for the web server
type Config struct {
	Addr   string
	API    *app.App
	Page   string // Liquipedia page path, used for webhook filtering
	Logger *slog.Logger
}

// Server is the HTTP server that handles webhook requests
type Server struct {
	api  *app.App
	page string // Liquipedia page path, used for webhook filtering
	log  *slog.Logger
}

// logger returns the server's logger, falling back to the global default when none was injected.
func (s *Server) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}

// Start initializes and starts the HTTP server with the given configuration
func Start(cfg Config) error {
	var serverLog *slog.Logger
	if cfg.Logger != nil {
		serverLog = cfg.Logger.With("component", "web")
	}
	s := &Server{
		api:  cfg.API,
		page: cfg.Page,
		log:  serverLog,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhooks/liquipedia", s.LiquipediaWebhookHandler)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	s.logger().Info("HTTP server listening", "addr", cfg.Addr)
	return srv.ListenAndServe()
}
