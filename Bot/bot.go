/* bot.go
 * Contains information running the bot. Requires a discord bot token, mongodb uri, format style (e.g. swiss) and liquipedia tournament url
 * Authors: Zachary Bower
 * Last modified: November 24th, 2024
 */

package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/bwmarrin/discordgo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var BotToken string
var Format string
var DbUri string
var LiquipediaURL string

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message")
	}
}

func Run() {
	// create a session
	discord, err := discordgo.New("Bot " + BotToken)
	checkNilErr(err)

	// add a event handler
	discord.AddHandler(newMessage)

	// open session
	discord.Open()
	defer discord.Close() // close session, after function termination

	// Use the SetServerAPIOptions() method to set the version of the Stable API on the client
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(DbUri).SetServerAPIOptions(serverAPI)
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

	// Get url for match page on liquipedia
	fmt.Println(LiquipediaURL)

	// keep bot running untill there is NO os interruption (ctrl + C)
	fmt.Println("Bot running....")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	/* prevent bot responding to its own message
	this is achived by looking into the message author id
	if message.author.id is same as bot.author.id then just return
	*/
	if message.Author.ID == discord.State.User.ID {
		return
	}

	// respond to user message if it contains one of the following commands
	switch {
	case strings.Contains(message.Content, "$check"):
		discord.ChannelMessageSend(message.ChannelID, "this feature hasn't been implemented yet")
	case strings.Contains(message.Content, "$help"):
		discord.ChannelMessageSend(message.ChannelID, getHelpMessage())
	case strings.Contains(message.Content, "$leaderboard"):
		discord.ChannelMessageSend(message.ChannelID, "this feature hasn't been implemented yet")
	case strings.Contains(message.Content, "$set"):
		discord.ChannelMessageSend(message.ChannelID, "this feature hasn't been implemented yet")
	case strings.Contains(message.Content, "$teams"):
		discord.ChannelMessageSend(message.ChannelID, "this feature hasn't been implemented yet")
	case strings.Contains(message.Content, "$upcoming"):
		discord.ChannelMessageSend(message.ChannelID, "this feature hasn't been implemented yet")
	case strings.Contains(message.Content, "$hello"):
		discord.ChannelMessageSend(message.ChannelID, "Hello World!")
	}
}

func getHelpMessage() string {
	var message = "PickEms Bot v2.0\n`$set [team1] [team2] ... [team10]`: Sets your Pick'Ems. 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams and 9-10 are the 0-3 teams. Please note that the teams names need to be specified exactly how the appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. \"The MongolZ\"\n`$check`: shows the current status of your Pick'Ems\n`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your PickEms\n`$leaderboard`: shows which users have the best pickems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes\n`$upcoming`: shows todays live and upcoming matches\n"
	return message
}
