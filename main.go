/* main.go
 * The "main" method for running the bot. For details about the bot see `readme.md`
 * Usage: go run main.go -format="<format>" -url="<url>"
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api "pickems-bot/api/api"
	bot "pickems-bot/bot"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get tournament name, round, page, params and test from .env file
	tournamentName := os.Getenv("TOURNAMENT_NAME")
	round := os.Getenv("ROUND")
	page := os.Getenv("PAGE")
	params := os.Getenv("PARAMS")
	test := os.Getenv("TEST")
	upcomingOnly := os.Getenv("UPCOMING_ONLY")

	// Default values if environmental variables not found
	if tournamentName == "" {
		tournamentName = "Blast_Austin_Major_2025"
	}
	if round == "" {
		round = "Stage_1"
	}
	if page == "" {
		page = "BLAST/Major/2025/Austin"
	}
	if test == "" {
		test = "false"
	}
	if upcomingOnly == "" {
		upcomingOnly = "false"
	}

	// Init API
	fullPage := fmt.Sprintf("%s/%s", page, round)
	apiInstance, err := api.NewAPI(tournamentName, os.Getenv("MONGO_PROD_URI"), fullPage, params, round)

	if err != nil {
		log.Fatalf("failed to initialize API: %v", err)
	}
	defer func() {
		if err = apiInstance.Store.Client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
	err = apiInstance.PopulateMatches()
	if err != nil {
		log.Fatal(err)
	}

	//Init bot
	var discordToken string
	if test == "false" { //Load production bot token
		discordToken = os.Getenv("DISCORD_PROD_TOKEN")
	} else if test == "true" {
		discordToken = os.Getenv("DISCORD_BETA_TOKEN")
	} else {
		fmt.Println("Invalid \"test\" value. Should be true or false")
	}
	botInstance, err := bot.NewBot(discordToken, apiInstance)

	// Run bot
	err = botInstance.Run()
	if err != nil {
		log.Fatal(fmt.Errorf("an unrecoverable error occured whilst running the bot: %w", err))
	}
}
