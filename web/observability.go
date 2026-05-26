package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pickems-bot/app"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TelemetryConfig holds the config for the telemetry server
type TelemetryConfig struct {
	Addr      string
	App       *app.App
	Discord   interface{ IsConnected() bool }
	StartTime time.Time
	Logger    *slog.Logger
}

// TelemetryServer is the HTTP server that handles telemetry (health + metrics) requests
type TelemetryServer struct {
	app       *app.App
	discord   interface{ IsConnected() bool }
	startTime time.Time
	log       *slog.Logger
}

// StartTelemetryServer initializes and starts the HTTP server with the passed config
func StartTelemetryServer(cfg TelemetryConfig) error {
	s := &TelemetryServer{
		app:       cfg.App,
		discord:   cfg.Discord,
		startTime: cfg.StartTime,
		log:       cfg.Logger,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", s.healthHandler)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	s.logger().Info("observability server listening", "addr", cfg.Addr)
	return srv.ListenAndServe()
}

// HealthResponse is the JSON response body returned by the /health endpoint.
type HealthResponse struct {
	Status string         `json:"status"`
	Checks ResponseChecks `json:"checks"`
	Uptime int64          `json:"uptime"`
}

// ResponseChecks holds the individual dependency check results within a HealthResponse.
type ResponseChecks struct {
	MongoDb string `json:"mongodb"`
	Discord string `json:"discord"`
}

func (s *TelemetryServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ok := true

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var mongodbStatus string
	if err := s.app.Store.Ping(ctx); err != nil {
		mongodbStatus = fmt.Sprintf("error: %v", err)
		ok = false
	} else {
		mongodbStatus = "ok"
	}

	var discordStatus string
	if s.discord.IsConnected() {
		discordStatus = "ok"
	} else {
		discordStatus = "disconnected"
		ok = false
	}

	var status string
	var statusCode int
	if ok {
		status = "ok"
		statusCode = http.StatusOK
	} else {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	response := HealthResponse{
		Status: status,
		Checks: ResponseChecks{
			MongoDb: mongodbStatus,
			Discord: discordStatus,
		},
		Uptime: int64(time.Since(s.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (s *TelemetryServer) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}
