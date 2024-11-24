/* main.go
 * The "main" method for running the bot. For details about the bot see `readme.md`
 * Usage: go run main.go -format="<format>" -url="<url>"
 * Authors: Zachary Bower
 * Last modified: November 24th, 2024
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	bot "pickems-bot/Bot"

	"github.com/joho/godotenv"
)

func main() {
	//Flags
	formatPtr := flag.String("format", "swiss", "Style of tournament, e.g. swiss, finals, iem")
	urlPtr := flag.String("url", "https://liquipedia.net/counterstrike/PGL/2024/Copenhagen", "Liquipedia Base URL: e.g. https://liquipedia.net/counterstrike/PGL/2024/Copenhagen")

	flag.Parse()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	discord_token := os.Getenv("DISCORD_PROD_TOKEN")
	uri := os.Getenv("MONGO_PROD_URI")

	//Init bot and run for tournament style
	if *formatPtr == "swiss" || *formatPtr == "finals" {
		bot.BotToken = discord_token
		bot.Format = *formatPtr
		bot.DbUri = uri
		bot.LiquipediaURL = *urlPtr
		bot.Run()
	} else {
		fmt.Println("Invalid tournament style... exiting")
	}
}
