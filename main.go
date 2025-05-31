/* main.go
 * The "main" method for running the bot. For details about the bot see `readme.md`
 * Usage: go run main.go -format="<format>" -url="<url>"
 * Authors: Zachary Bower
 * Last modified: 29/05/2025
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	bot "pickems-bot/Bot"
	api "pickems-bot/api/api"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	
	//Flags
	roundPtr := flag.String("round", "Stage_1", "Round of tournament (Stage_1, Opening, Playoffs, etc)")
	tournamentNamePtr := flag.String("tournamentName", "AustinMajor2025", "Tournament name, e.g. AustinMajor2025")
	pagePtr := flag.String("tournamentPage", "BLAST/Major/2025/Austin", "Liquipedia Wiki Page: e.g. BLAST/Major/2025/Austin")
	paramsPtr := flag.String("optionalParams", "", "Optional params required by some tournament format, if unsure leave empty")
	testPtr := flag.String("test", "false", "Use release or beta bot: takes true or false as argument")

	flag.Parse()
	
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Init API
	page := fmt.Sprintf("%s/%s", *pagePtr, *roundPtr) 
	apiInstance, err := api.NewAPI(*tournamentNamePtr, os.Getenv("MONGO_PROD_URI"), page, *paramsPtr, *roundPtr)
	
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
	if *testPtr == "false" { //Load production bot token
		discordToken = os.Getenv("DISCORD_PROD_TOKEN")
	} else if *testPtr == "true" {
		discordToken = os.Getenv("DISCORD_BETA_TOKEN")
	} else {
		fmt.Println("Invalid \"test\" flag. Should be true or false")
	}
	botInstance, err := bot.NewBot(discordToken, apiInstance)
	
	// Run bot
	err = botInstance.Run()
	if err != nil {
		log.Fatal(fmt.Errorf("an unrecoverable error occured whilst running the bot: %w", err))
	}
}
