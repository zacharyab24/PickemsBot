/* main.go
 * The "main" method for running the bot. For details about the bot see `readme.md`
 * Usage: go run main.go -format="<format>" -url="<url>"
 * Authors: Zachary Bower
 * Last modified: November 28/05/2025
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	bot "pickems-bot/Bot"
	match "pickems-bot/api/match"

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
	// // Match Results
	// result, err := api.GetMatchData("BLAST/Major/2025/Austin/Stage_1", "")
	// //result, err := api.GetMatchData("Galaxy_Battle/2025/Phase_2", "&section=24")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	
	// switch r := result.(type) {
    // case match.SwissResult:
    //     fmt.Println("Swiss tournament results:")
    //     for team, score := range r.Scores {
    //         fmt.Printf("%s: %s\n", team, score)
    //     }
    // case match.EliminationResult:
    //     fmt.Println("Elimination tournament results:")
    //     for team, progression := range r.Progression {
    //         fmt.Printf("%s: %s[%s]\n", team, progression.Round, progression.Status)
    //     }
    // }
	// fmt.Println()

	// // Upcoming Matches
	// matches, err := api.GetUpcomingMatchData("BLAST/Major/2025/Austin/Stage_1", "")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// for _, match := range matches {
	// 	fmt.Printf("%s VS %s (Bo%s): %d: %s\n", match.Team1, match.Team2, match.BestOf, match.EpochTime, match.StreamUrl)
	// }

	// fmt.Println("Storing results in DB...")
	// err = match.StoreMatchResults("test", "test", result, "test", matches)
	// if err != nil {
	// 	fmt.Printf("error storing results in db: %v", err)
	// }
	//Test db fetching
	res, err := match.FetchMatchResultsFromDb("test", "test", "test")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res)


}
