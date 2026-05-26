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

	apiInstance, err := app.NewApp(cfg, os.Getenv("MONGO_PROD_URI"), logger)
	if err != nil {
		logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err = apiInstance.Store.GetClient().Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	logger.Debug("populating matches from data source", "source", cfg.DataSource, "schedule_only", cfg.UpcomingOnly)
	if err := apiInstance.PopulateMatches(cfg.UpcomingOnly); err != nil {
		logger.Error("failed to populate matches", "error", err)
		os.Exit(1)
	}
	logger.Debug("match data populated")

	if err := apiInstance.GenerateLeaderboard(); err != nil {
		logger.Error("failed to generate leaderboard", "error", err)
		os.Exit(1)
	}
	logger.Debug("leaderboard generated")

	// Regenerate the result image on startup so it always reflects the current
	// tournament/bracket. The image on disk can be stale if the bot was previously
	// run against a different tournament and the file was not cleared between restarts.
	// Fatal in full mode — $results will be broken if this fails. In upcoming_only
	// mode there are no match results yet so this is expected to fail; skip it.
	if !cfg.UpcomingOnly {
		if err := web.RenderResultsImage(apiInstance); err != nil {
			logger.Error("could not render results image on startup", "error", err)
			os.Exit(1)
		}
		logger.Debug("results image rendered")
	}

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
		poller := web.NewPoller(apiInstance, cfg.PandaScore.SeriesID, os.Getenv("PANDASCORE_API_KEY"), cfg.PandaScore.APIURL, logger)
		go poller.Start()
		logger.Info("PandaScore poller started")
	case "liquipedia":
		go func() {
			if err := web.Start(web.Config{Addr: ":8080", API: apiInstance, Page: cfg.Liquipedia.Page, Logger: logger}); err != nil {
				logger.Error("web server exited", "error", err)
				os.Exit(1)
			}
		}()
		logger.Info("Liquipedia webhook server starting", "addr", ":8080")
	default:
		logger.Error("unknown data_source in config.toml", "data_source", cfg.DataSource)
		os.Exit(1)
	}

	if err := botInstance.Run(); err != nil {
		logger.Error("unrecoverable error running the bot", "error", fmt.Errorf("bot.Run: %w", err))
		os.Exit(1)
	}
}
