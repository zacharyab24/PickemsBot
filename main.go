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

	api "pickems-bot/api/api"
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

	apiInstance, err := api.NewAPI(cfg.TournamentName, os.Getenv("MONGO_PROD_URI"), cfg.Page, cfg.Params, cfg.Round)
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
	if err := web.RenderResultsImage(apiInstance); err != nil {
		log.Printf("warning: could not render results image on startup: %v", err)
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

	go func() {
		if err := web.Start(web.Config{Addr: ":8080", API: apiInstance}); err != nil {
			log.Fatalf("failed to start web server: %v", err)
		}
	}()

	if err := botInstance.Run(); err != nil {
		log.Fatal(fmt.Errorf("an unrecoverable error occured whilst running the bot: %w", err))
	}
}
