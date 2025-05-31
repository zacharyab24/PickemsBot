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
	"strings"

	bot "pickems-bot/Bot"
	api "pickems-bot/api/api"
	"pickems-bot/api/shared"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
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

	// API Testing
	ApiTesting(apiInstance)

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
	botInstance.Run()
}

// This provides a sample of how the api functions work and how they can be incorporated into bot
func ApiTesting(api *api.API) {
	user := shared.User{UserId: "321asdf", Username: "321yasdf"}
	// userPreds := []string{
	// 	"Natus Vincere",
	// 	"Team Vitality",
	// 	"Team Spirit",
	// 	"MOUZ",
	// 	"FaZe Clan",
	// 	"G2 Esports",
	// 	"Team Liquid",
	// 	"The MongolZ",
	// 	"3DMAX",
	// 	"GamerLegion",
	// }
	userPreds := []string {
		"Mouz",
		"G2",
		"faze",
		"spirit",
	}


	fmt.Println("[Populating scheduled matches]")
	err := api.PopulateMatches()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println()

	fmt.Println("[Getting teams list]")
	teams, err := api.GetTeams()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Valid teams for this stage are:")
	for _, team := range teams {
		fmt.Printf("- %s\n", team)
	}
	fmt.Println()
	
	fmt.Println("[Getting upcoming matches]")
	matches, err := api.GetUpcomingMatches()
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(matches) == 0 {
		fmt.Println("No upcoming matches")
	} else {
		fmt.Println("Upcoming matches:")
		for _,match := range matches {
			if strings.Contains(match, "TBD") {
				continue
			}
			fmt.Print(match)
		}
	}
	fmt.Println()

	fmt.Println("[Setting user prediction]")
	err = api.SetUserPrediction(user, userPreds ,api.Store.Round)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%s's Pickems have been updated\n", user.Username)
	fmt.Println()
	
	fmt.Println("[Checking user prediction]")
	report, err := api.CheckPrediction(user)
	dontPrint := false
	if err != nil {
		if err == mongo.ErrNoDocuments {
			dontPrint = true
		} else {
			fmt.Println(err)
			return
		}
	}
	if dontPrint {
		fmt.Printf("%s does not have any Pickems stored. Use $set to set your predictions\n", user.Username)
	} else {
		fmt.Println(report)
	}
	fmt.Println()

	fmt.Println("[Getting leaderboard]")
	leaderboard, err := api.GetLeaderboard() 
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(leaderboard)

}
