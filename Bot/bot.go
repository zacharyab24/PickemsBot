/* bot.go
 * Contains information running the bot. Requires a discord bot token, mongodb uri, format style (e.g. swiss) and liquipedia tournament url
 * Authors: Zachary Bower
 * Last modified: November 24th, 2024
 */

/* TODO:
 * Update to work with finals bot
 * Testing with check for finals. Can't test this as dont have the required data. Can check when finals teams become available
 * Convert team names to lower before putting the the db or checking. Removed this from getValidTeams() so it looked nicer for user
 */
package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"pickems-bot/api/api"
	"strings"

	"github.com/bwmarrin/discordgo"
)



type Bot struct {
	BotToken string
	ApiPtr *api.API
}

func NewBot(botToken string, apiPtr *api.API) (*Bot, error) {
	if botToken == ""{
		return nil, fmt.Errorf("botToken is required but none was provided")
	}

	return &Bot{
		BotToken: botToken,
		ApiPtr: apiPtr,
	}, nil
}

func (b *Bot) Run() {
	// create a session
	discord, err := discordgo.New("Bot " + b.BotToken)
	checkNilErr(err)

	// add a event handler
	discord.AddHandler(newMessage)

	// open session
	discord.Open()
	defer discord.Close() // close session, after function termination

	// keep bot running until there is NO os interruption (ctrl + C)
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
	case startsWith(message.Content, "$check"):
		checkPredictions(discord, message)
	case startsWith(message.Content, "$help"):
		discord.ChannelMessageSend(message.ChannelID, getHelpMessage())
	case startsWith(message.Content, "$leaderboard"):
		getLeaderboard(discord, message)
	case startsWith(message.Content, "$set"):
		setPredictions(discord, message)
	case startsWith(message.Content, "$teams"):
		discord.ChannelMessageSend(message.ChannelID, getTeamsMessage())
	case startsWith(message.Content, "$upcoming"):
		getUpcomingMatches(discord, message)
	case startsWith(message.Content, "$hello"):
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Hello %s!", message.Author.Username))
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Hello %s!", message.Author.GlobalName))
	}
}

// Function to return the help message called by `$help`
// Preconditions: None
// Postconditions: Returns a string containing the entire help message
func getHelpMessage() string {
	var message = "PickEms Bot v2.0\n`$set [team1] [team2] ... [team10]`: Sets your Pick'Ems. 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams and 9-10 are the 0-3 teams. Please note that the teams names need to be specified exactly how the appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. \"The MongolZ\"\n`$check`: shows the current status of your Pick'Ems\n`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your PickEms\n`$leaderboard`: shows which users have the best pickems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes\n`$upcoming`: shows todays live and upcoming matches\n"
	return message
}

// Helper function to check if a string starts with a given substring
// Preconditions: Recieves an input string and a substring
// Postconditions: Returns true if the substring is at the start of the string, else returns false
func startsWith(inputString string, substring string) bool {
	//Check if the substring is present in the input string
	if !strings.Contains(inputString, substring) {
		return false
	}
	strLength := len(substring)
	for i := range strLength {
		if inputString[i] != substring[i] {
			return false
		}
	}
	return true
}

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message")
	}
}