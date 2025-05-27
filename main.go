/* main.go
 * The "main" method for running the bot. For details about the bot see `readme.md`
 * Usage: go run main.go -format="<format>" -url="<url>"
 * Authors: Zachary Bower
 * Last modified: November 24th, 2024
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"pickems-bot/Api"
	bot "pickems-bot/Bot"

	"github.com/joho/godotenv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// API Testing
	ApiTesting()
	os.Exit(1)
	
	//Flags
	formatPtr := flag.String("format", "swiss", "Style of tournament, e.g. swiss, finals, iem")
	roundPtr := flag.String("round", "opening", "Round of tournament (opening, elimination, playoffs)")
	tournamentNamePtr := flag.String("tournamentName", "ShanghaiMajor2024", "Tournament name, e.g. ShanghaiMajor2024")
	urlPtr := flag.String("url", "https://liquipedia.net/counterstrike/Perfect_World/Major/2024/Shanghai", "Liquipedia Base URL: e.g. https://liquipedia.net/counterstrike/PGL/2024/Copenhagen")
	testPtr := flag.String("test", "false", "Use main or test bot: takes true or false as argument")

	flag.Parse()

	err := godotenv.Load()
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
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)
	
	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
	// Send a ping to confirm a successful connection
	if err := client.Database("admin").RunCommand(context.TODO(), bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
		panic(err)
	}
	fmt.Println("Successfully connected to MongoDB!")

	//Init bot and run for tournament style
	if *formatPtr == "swiss" || *formatPtr == "finals" {
		bot.BotToken = discordToken
		bot.Format = *formatPtr
		bot.Round = *roundPtr
		bot.TournamentName = *tournamentNamePtr
		bot.Client = client
		bot.LiquipediaURL = *urlPtr
		bot.Run()
	} else {
		fmt.Println("Invalid tournament style... exiting")
	}
}

func ApiTesting() {
	// page := "Galaxy_Battle/2025/Phase_2"
	// param := "&section=24"
	
	// page := "Perfect_World/Major/2024/Shanghai/Opening_Stage"
	// param := ""
	// //page := "BLAST/Major/2025/Austin/Playoffs"
	// url := fmt.Sprintf("https://liquipedia.net/counterstrike/%s?action=raw%s", page, param)
	
	// wikitext, err := Api.GetWikitext(url)
	// if err != nil {
	// 	fmt.Println("An error occured whilst fetching match2bracketid data: ", err)
	// 	return
	// }

	// ids, err := Api.ExtractMatchListId(wikitext)
	// if err != nil {
	// 	fmt.Println("An error occured:", err)
	// 	return
	// }

	// //Func to get JSON data
	// liquipediaDBApiKey := os.Getenv("LIQUIDPEDIADB_API_KEY")
	// apiRequestString := "https://api.liquipedia.net/api/v3/match"
	// jsonResponse, err := Api.GetLiquipediaMatchData(liquipediaDBApiKey, ids, apiRequestString)
	// if err != nil {
	// 	fmt.Println("An error occured whilst fetching match data")
	// 	return
	// }

	// Load data from file instead of API request
	data, err := os.ReadFile("Api/pw_response.json")
	if err != nil {
		fmt.Println("An error occured opening the file:",err)
		return
	}
	jsonResponse := string(data)
	matchNodes, err := Api.GetMatchNodesFromJson(jsonResponse)
	if err != nil {
		fmt.Println("An error parsing match data", err)
		return
	}

	// scores, err := Api.CalculateSwissScores(matchNodes)
	// if err != nil {
	// 	fmt.Println("An error occured whilst parsing match data")
	// }
	// for _, team := range scores {
	// 	fmt.Printf("%s: %s\n", team, scores[team])
	// }

	tree, err := Api.GetMatchTree(matchNodes)
	if err != nil {
		fmt.Println("An error occured whilst parsing match data")
		return
	}
	fmt.Println(tree)
}
