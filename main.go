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
	api, err := api.NewAPI("test", os.Getenv("MONGO_PROD_URI"), "BLAST/Major/2025/Austin/Stage_1", "", "Stage_1")
	//api, err := api.NewAPI("test", os.Getenv("MONGO_PROD_URI"), "Perfect_World/Major/2024/Shanghai/Playoff_Stage", "", "Playoff_Stage")
	if err != nil {
		log.Fatalf("failed to initialize API: %v", err)
	}	
	defer func() {
		if err = api.Store.Client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	// API Testing
	ApiTesting(api)

	//Init bot and run for tournament style
	if *formatPtr == "swiss" || *formatPtr == "finals" {
		bot.BotToken = discordToken
		bot.Format = *formatPtr
		bot.Round = *roundPtr
		bot.TournamentName = *tournamentNamePtr
		bot.Client = api.Store.Client
		bot.LiquipediaURL = *urlPtr
		bot.Run()
	} else {
		fmt.Println("Invalid tournament style... exiting")
	}
}

// This provides a sample of how the api functions work and how they can be incorporated into bot
func ApiTesting(api *api.API) {
	user := shared.User{UserId: "123", Username: "123x"}
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
		"vitality",
	}


	fmt.Println("Populating scheduled matches")
	err := api.PopulateMatches()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Getting teams list")
	teams, err := api.GetTeams()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(teams)

	fmt.Println("Getting upcoming matches")
	matches, err := api.GetUpcomingMatches()
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(matches) == 0 {
		fmt.Println("No upcoming matches")
	} else {
		for _,match := range matches {
			if strings.Contains(match, "TBD") {
				continue
			}
			fmt.Print(match)
		}
	}

	fmt.Println("Setting user prediction")
	err = api.SetUserPrediction(user, userPreds ,api.Store.Round)
	if err != nil {
		fmt.Println(err)
		return
	}
	
	fmt.Println("Checking user prediction")
	report, err := api.CheckPrediction(user)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(report)

	fmt.Println("Getting leaderboard")


}
