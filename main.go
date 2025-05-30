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
	processing "pickems-bot/api/input_processing"
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
	processing.Init(uri)
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
	// page := "BLAST/Major/2025/Austin/Stage_1"
	// params := ""
	page := "Perfect_World/Major/2024/Shanghai/Elimination_Stage"
	params := ""
	
	dbName := "test"
	round := "Elimination_Stage"
	
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

	// Get Match Results from DB
	fmt.Println("Running `GetMatchResults`")
	results, err := api.GetMatchResults(dbName, "test", round, page, params)
	if err != nil {
		fmt.Println(err)
		return
	}
	//fmt.Println(results)

	//Valid team name lookup
	fmt.Println("Running `GetValidTeams`")
	teams, format, err := api.GetValidTeams(dbName, "test", round)
	if err != nil {
		fmt.Println(err)
		return
	} 

	var requiredPredictions int
	switch format {
	case "swiss":
		requiredPredictions = 10
	case "single-elimination" :
		T := len(teams)
		requiredPredictions = T / 2
	default:
		requiredPredictions = 0
		fmt.Printf("unknown tournament format: %s\n", format)
	}

	// // // Input teams:
	// // input := []string{
	// // 	"MOUZ", "G2", "FaZe", "Team Spirit",
	// // }
	input := []string{
		"natus vincere",
		"team vitality",
		"team spirit",
		"mouz",
		"faze clan",
		"g2 esports",
		"team liquid",
		"the mongolz",
		"3dmax",
		"gamerlegion",
	}

	//fmt.Println(teams)

	fmt.Println("Checking if input teams are valid")
	validTeams, invalidTeams := processing.CheckTeamNames(input, teams)
	if invalidTeams == nil {
		fmt.Println("All team names are valid")
	} else {
		fmt.Println("The following team names are invalid:")
		for i := range invalidTeams {
			fmt.Println(invalidTeams[i])
		}
		return
	}

	// Need to make sure that what we pass into GeneratePrediction is the actual team name, not the fuzzy matched name

	// Test prediction store and lookup
	// User hard coded to my discord user for testing
	user := processing.User{UserId: "123", Username: "123x"}
	fmt.Println("Generating prediction")
	prediction, err := processing.GeneratePrediction(user, format, round, validTeams, requiredPredictions)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(prediction)

	// Insert prediction into db
	fmt.Println("Inserting prediction to db")
	err = processing.StoreUserPrediction("test", "test_user_predictions", user.UserId, prediction)
	if err != nil {
		fmt.Println(err)
	}

	// Fetch prediction from db
	
	// Test fetching prediction from db:
	fmt.Println("Fetching a single prediction document from db")
	doc, err := processing.GetUserPrediction("test", "test_user_predictions", "123", round)
	if err != nil {
		fmt.Println(err)
	}
	//fmt.Println(doc)

	// Test calculate user score:
	fmt.Println("Checking user score")
	score, err := processing.CalculateUserScore(doc, results)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Results for %s: Successes: %d, Failures: %d, Pending: %d\n", user.Username, score.Successes, score.Failed, score.Pending)

	// fmt.Println("Fetching all documents for round")
	// newDoc, err := processing.GetAllUserPredictions("test", "test_user_predictions", round)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// for i := range newDoc {
	// 	fmt.Println(newDoc[i])
	// }

}
