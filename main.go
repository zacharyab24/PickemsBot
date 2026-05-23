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
	"log"
	"os"

	"pickems-bot/app"
	bot "pickems-bot/bot"
	"pickems-bot/config"
	"pickems-bot/web"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	apiInstance, err := app.NewApp(cfg, os.Getenv("MONGO_PROD_URI"))
	if err != nil {
		log.Fatalf("failed to initialize API: %v", err)
	}
	defer func() {
		if err = apiInstance.Store.GetClient().Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	if err := apiInstance.PopulateMatches(cfg.UpcomingOnly); err != nil {
		log.Fatal(err)
	}
	if err := apiInstance.GenerateLeaderboard(); err != nil {
		log.Fatal(err)
	}

	// Regenerate the result image on startup so it always reflects the current
	// tournament/bracket. The image on disk can be stale if the bot was previously
	// run against a different tournament and the file was not cleared between restarts.
	// Fatal in full mode — $results will be broken if this fails. In upcoming_only
	// mode there are no match results yet so this is expected to fail; skip it.
	if !cfg.UpcomingOnly {
		if err := web.RenderResultsImage(apiInstance); err != nil {
			log.Fatalf("could not render results image on startup: %v", err)
		}
	}

	var discordToken string
	if cfg.Test {
		discordToken = os.Getenv("DISCORD_BETA_TOKEN")
	} else {
		discordToken = os.Getenv("DISCORD_PROD_TOKEN")
	}
	botInstance, err := bot.NewBot(discordToken, apiInstance)
	if err != nil {
		log.Fatal(err)
	}

	switch cfg.DataSource {
	case "pandascore":
		poller := web.NewPoller(apiInstance, cfg.SeriesID, os.Getenv("PANDASCORE_API_KEY"))
		go poller.Start()
		log.Println("PandaScore poller started")
	case "liquipedia":
		go func() {
			if err := web.Start(web.Config{Addr: ":8080", API: apiInstance, Page: cfg.Page}); err != nil {
				log.Fatalf("failed to start web server: %v", err)
			}
		}()
		log.Println("Liquipedia webhook server starting on :8080")
	default:
		log.Fatalf("unknown data_source %q in config.toml", cfg.DataSource)
	}

	if err := botInstance.Run(); err != nil {
		log.Fatal(fmt.Errorf("an unrecoverable error occured whilst running the bot: %w", err))
	}
}
