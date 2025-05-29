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
	"pickems-bot/api"
	match "pickems-bot/api/match_data"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	
	//Flags
	formatPtr := flag.String("format", "swiss", "Style of tournament, e.g. swiss, finals, iem")
	roundPtr := flag.String("round", "opening", "Round of tournament (opening, elimination, playoffs)")
	tournamentNamePtr := flag.String("tournamentName", "ShanghaiMajor2024", "Tournament name, e.g. ShanghaiMajor2024")
	urlPtr := flag.String("url", "https://liquipedia.net/counterstrike/Perfect_World/Major/2024/Shanghai", "Liquipedia Base URL: e.g. https://liquipedia.net/counterstrike/PGL/2024/Copenhagen")
	testPtr := flag.String("test", "false", "Use main or test bot: takes true or false as argument")

	flag.Parse()
	
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	var discordToken string
	if *testPtr == "false" { //Load production bot token
		discordToken = os.Getenv("DISCORD_PROD_TOKEN")
	} else if *testPtr == "true" {
		discordToken = os.Getenv("DISCORD_BETA_TOKEN")
	} else {
		fmt.Println("Invalid \"test\" flag. Should be true or false")
	}

	// MongoDB Stuff
	uri := os.Getenv("MONGO_PROD_URI")
	match.Init(uri)
	defer func() {
		if err = match.Client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	// API Testing
	ApiTesting()

	//Init bot and run for tournament style
	if *formatPtr == "swiss" || *formatPtr == "finals" {
		bot.BotToken = discordToken
		bot.Format = *formatPtr
		bot.Round = *roundPtr
		bot.TournamentName = *tournamentNamePtr
		bot.Client = match.Client
		bot.LiquipediaURL = *urlPtr
		bot.Run()
	} else {
		fmt.Println("Invalid tournament style... exiting")
	}
}

// This provides a sample of how the api functions work and how they can be incorporated into bot
func ApiTesting() {
	page := "BLAST/Major/2025/Austin/Stage_1"
	params := ""
	dbName := "test"
	round := "stage_1"
	
	// Get Matches for this stage (this should be run on app start up)
	fmt.Println("Getting upcoming matches from LiquipediaDB Api")
	upcomingMatches, err := api.FetchUpcomingMatches(page, params)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Store Matches in DB (this should be run on startup)
	fmt.Println("Storing upcoming matches in db")
	err = match.StoreUpcomingMatches(dbName, "upcoming_matches", round, upcomingMatches)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Running `GetMatchResults`")
	results, err := api.GetMatchResults(dbName, "test", round, page, params)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(results)
}
