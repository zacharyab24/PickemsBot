//go:build !test

/* main.go
 * The "main" method for running the bot. For details about the bot see `readme.md`
 * Tournament settings live in config.toml (see `go run ./scripts/configure`).
 * Secrets (Discord tokens, Mongo URI, Liquipedia API key) live in .env.
 * Authors: Zachary Bower
 */

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"pickems-bot/app"
	bot "pickems-bot/bot"
	"pickems-bot/config"
	"pickems-bot/web"

	"github.com/joho/godotenv"
)

func main() {
	startTime := time.Now()

	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, using environment variables")
	}

	cfg, err := config.Load("config.toml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Determine log level: explicit config value wins; otherwise debug in test
	// mode and info in production.
	level := slog.LevelInfo
	if cfg.LogLevel != "" {
		if parseErr := level.UnmarshalText([]byte(cfg.LogLevel)); parseErr != nil {
			slog.Warn("unrecognised log_level in config.toml, defaulting to info", "value", cfg.LogLevel)
			level = slog.LevelInfo
		}
	} else if cfg.Test {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	apiInstance, err := app.NewApp(cfg, os.Getenv("POSTGRES_URI"), logger)
	if err != nil {
		logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}
	defer apiInstance.Store.Close()

	var discordToken string
	if cfg.Test {
		discordToken = os.Getenv("DISCORD_BETA_TOKEN")
	} else {
		discordToken = os.Getenv("DISCORD_PROD_TOKEN")
	}
	botInstance, err := bot.NewBot(discordToken, apiInstance, logger)
	if err != nil {
		logger.Error("failed to initialize bot", "error", err)
		os.Exit(1)
	}

	go func() {
		if err := web.StartTelemetryServer(web.TelemetryConfig{
			Addr:      ":9090",
			App:       apiInstance,
			Discord:   botInstance,
			StartTime: startTime,
			Logger:    logger,
		}); err != nil {
			logger.Error("telemetry server exited", "error", err)
			os.Exit(1)
		}
	}()

	switch cfg.DataSource {
	case "pandascore":
		externalID := strconv.Itoa(cfg.PandaScore.TournamentID)
		dbTournamentID, err := apiInstance.Store.EnsureTournament(context.Background(), externalID, "pandascore", cfg.TournamentName, cfg.PandaScore.SeriesID)
		if err != nil {
			logger.Error("failed to ensure tournament in database", "error", err)
			os.Exit(1)
		}
		if err := apiInstance.PopulateMatches(context.Background(), dbTournamentID, cfg.Round, false); err != nil {
			logger.Warn("startup populate failed, bot will retry on next poller tick", "error", err)
		}
		poller := web.NewPoller(apiInstance, cfg.PandaScore.SeriesID, cfg.PandaScore.TournamentID, dbTournamentID, cfg.Round, os.Getenv("PANDASCORE_API_KEY"), cfg.PandaScore.APIURL, logger)
		go poller.Start()
		logger.Info("PandaScore poller started", "db_tournament_id", dbTournamentID)
	case "liquipedia":
		dbTournamentID, err := apiInstance.Store.EnsureTournament(context.Background(), cfg.Liquipedia.Page, "liquipedia", cfg.TournamentName, 0)
		if err != nil {
			logger.Error("failed to ensure tournament in database", "error", err)
			os.Exit(1)
		}
		go func() {
			if err := web.Start(web.Config{Addr: ":8080", API: apiInstance, Page: cfg.Liquipedia.Page, TournamentID: dbTournamentID, Round: cfg.Round, Logger: logger}); err != nil {
				logger.Error("web server exited", "error", err)
				os.Exit(1)
			}
		}()
		logger.Info("Liquipedia webhook server starting", "addr", ":8080", "db_tournament_id", dbTournamentID)
	default:
		logger.Error("unknown data_source in config.toml", "data_source", cfg.DataSource)
		os.Exit(1)
	}

	if err := botInstance.Run(); err != nil {
		logger.Error("unrecoverable error running the bot", "error", fmt.Errorf("bot.Run: %w", err))
		os.Exit(1)
	}
}
