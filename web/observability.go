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
	mux.HandleFunc("/poller", s.pollerStatusHandler)

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

// requireGET rejects anything but GET/HEAD, writing 405 and returning false
// if so. Callers should return immediately when this returns false.
func requireGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func (s *TelemetryServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
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

// PollerStatusResponse is the JSON response body returned by the /poller endpoint.
type PollerStatusResponse struct {
	Enabled    bool                `json:"enabled"`
	Message    string              `json:"message,omitempty"`
	LastPollAt *time.Time          `json:"last_poll_at,omitempty"`
	Tracked    []TrackedTournament `json:"tracked_tournaments"`
}

// TrackedTournament is one tournament currently tracked by the poller.
type TrackedTournament struct {
	TournamentID           int    `json:"tournament_id"`
	Name                   string `json:"name"`
	PandascoreTournamentID int    `json:"pandascore_tournament_id"`
	SeriesID               int    `json:"series_id"`
	Round                  string `json:"round"`
}

// pollerStatusHandler reports the poller's monitoring pool contents and last
// poll time. The poller always runs (see main.go) regardless of the
// deployment's configured data_source - guild_config is source-agnostic, so
// it's not gated on that value; an empty tracked_tournaments list already
// conveys "nothing to poll right now".
func (s *TelemetryServer) pollerStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}

	w.Header().Set("Content-Type", "application/json")

	entries := s.app.Snapshot()
	tracked := make([]TrackedTournament, 0, len(entries))
	for _, e := range entries {
		tracked = append(tracked, TrackedTournament{
			TournamentID:           e.DBTournamentID,
			Name:                   e.Name,
			PandascoreTournamentID: e.PandascoreTournamentID,
			SeriesID:               e.SeriesID,
			Round:                  e.Round,
		})
	}

	response := PollerStatusResponse{
		Enabled: true,
		Tracked: tracked,
	}
	if lastPoll, ok := s.app.LastPollTime(); ok {
		response.LastPollAt = &lastPoll
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (s *TelemetryServer) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}
