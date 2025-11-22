/* bot.go
 * Contains logic used for creating and running the bot. Requires a discord bot token, and ApiPtr both of which are
 * passed in from main.go
 * Authors: Zachary Bower
 */

package bot

import (
	"fmt"
	"os"
	"os/signal"
	"pickems-bot/api/api"
	"pickems-bot/api/shared"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/go-andiamo/splitter"
	"go.mongodb.org/mongo-driver/mongo"
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

func (b *Bot) Run() error {
	// create a session
	discord, err := discordgo.New("Bot " + b.BotToken)
	if err != nil {
		return err
	}

	// add an event handler
	discord.AddHandler(b.newMessage)

	// open session
	discord.Open()
	defer discord.Close() // close session, after function termination

	// keep bot running until there is NO os interruption (ctrl + C)
	fmt.Println("Bot running....")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	return nil
}

func (b *Bot) newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	//To prevent bot from responding to its own message, if the message author id matches the bot's then just return
	if message.Author.ID == discord.State.User.ID {
		return
	}

	// respond to user message if it contains one of the following commands
	switch {
	case startsWith(message.Content, "$help"):
		b.helpMessage(discord, message)

	case startsWith(message.Content, "$details"):
		b.details(discord, message)

	case startsWith(message.Content, "$set"):
		b.setPredictions(discord, message)

	case startsWith(message.Content, "$check"):
		b.checkPredictions(discord, message)

	case startsWith(message.Content, "$leaderboard"):
		b.leaderboard(discord, message)

	case startsWith(message.Content, "$teams"):
		b.teams(discord, message)
		
	case startsWith(message.Content, "$upcoming"):
		b.upcomingMatches(discord, message)
	}
}

// Function to prints the help message called by `$help` to a discord channel
// Preconditions: None
// Postconditions: Help message is sent to the discord channel
func (b *Bot) helpMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	var res strings.Builder
	res.WriteString("PickEms Bot v3.0\n") 
	res.WriteString("`details`: Get information about the tournament including name, round, format, and number of required teams for setting prediction\n") 
	res.WriteString("`$set team1 ... teamN`: Sets your Pick'Ems\n")
	res.WriteString("For a swiss tournament, 10 teams are required: 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams and 9-10 are the 0-3 teams.\n")
	res.WriteString("For a single elimination tournament, 4 teams are required: 1 & 2 are the teams that place 3rd and 4th in the tournament, 3 is the team that places 2nd and 4 is the team that places first\n")
	res.WriteString("There is fuzzy matching on names, however you should try and have a close match for the best results. Names that contain two or more words need to be encase in \" (e.g. \"The MongolZ\")\n") 
	res.WriteString("`$check`: shows the current status of your Pick'Ems\n")
	res.WriteString("`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your PickEms\n")
	res.WriteString("`$leaderboard`: shows which users have the best pickems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes\n")
	res.WriteString("`$upcoming`: shows the upcoming matches for this round of the tournament with confirmed teams\n") 
	discord.ChannelMessageSend(message.ChannelID, res.String())
}

// Function that prints the `$details` message in a discord channel
// Preconditions: Receives pointer to discordgo session and discordgo message
// Postconditions: User predictions are updated if data is valid, else an error message is sent to the discord channel
func (b *Bot) details(discord *discordgo.Session, message *discordgo.MessageCreate) {
	info, err := b.ApiPtr.GetTournamentInfo()
	if err != nil {
		fmt.Println(err)
		discord.ChannelMessageSend(message.ChannelID, "An unexpected error occured")
	}
	var res strings.Builder
	for i := range info {
		res.WriteString(fmt.Sprintf("%s\n",info[i]))
	}
	discord.ChannelMessageSend(message.ChannelID, res.String())
	
}

// Function that processes the user input for `$set` message, validates the picks are correct and updates the values stored in the db
// Preconditions: Receives pointer to discordgo session and discordgo message
// Postconditions: User predictions are updated if data is valid, else an error message is sent to the discord channel
func (b *Bot) setPredictions(discord *discordgo.Session, message *discordgo.MessageCreate) {
	user := shared.User{UserId: message.Author.ID, Username: message.Author.Username}
	res := fmt.Sprintf("%s's Pickems have been updated\n", user.Username)

	// Get User Predictions from message
	spaceSplitter, _ := splitter.NewSplitter(' ', splitter.DoubleQuotes, splitter.LeftRightDoubleDoubleQuotes) //we use splitter here instead of go's build in splitter because now we can have team names that contain spaces e.g. "Faze Clan" recognised as one team not two
	msg, _ := spaceSplitter.Split(message.Content)
	userPreds := msg[1:]

	err := b.ApiPtr.SetUserPrediction(user, userPreds ,b.ApiPtr.Store.Round)
	if err != nil {
		fmt.Println(err)
		res = fmt.Sprintf("An error occured setting %s's Pickems: %s", user.Username, err)
	}	
	discord.ChannelMessageSend(message.ChannelID, res)
}

// Function to check the current status of a user's predictions
// Preconditions: Receives pointer to the discordgo session and discordgo message
// Postconditions: Sends the status of the users's predictions to the discord channel in the form 
// "Succeeded: {succeeded}, Failed: {failed}, Pending: {pending}"
func (b *Bot) checkPredictions(discord *discordgo.Session, message *discordgo.MessageCreate) {
	user := shared.User{UserId: message.Author.ID, Username: message.Author.Username}
	res, err := b.ApiPtr.CheckPrediction(user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			res = fmt.Sprintf("%s does not have any Pickems stored. Use $set to set your predictions\n", user.Username)
		} else {
			fmt.Println(err)
			res = fmt.Sprintf("An error occured checking %s's Pickems", user.Username)
		}
	} 
	discord.ChannelMessageSend(message.ChannelID, res)
}

// Function to calculate the leaderboard and send the leaderboard to a discord channel
// Preconditions: Receives pointer to the discordgo session and discordgo message
// Postconditions: Generates leaderboard and posts leaderboard to same channel command was run
func (b *Bot) leaderboard(discord *discordgo.Session, message *discordgo.MessageCreate) {
	res, err := b.ApiPtr.GetLeaderboard() 
	if err != nil {
		fmt.Println(err)
		res = "An error occured getting the leaderboard"
	}
	discord.ChannelMessageSend(message.ChannelID, res)
}

// Function to get valid teams and send the result to a discord channel 
// Preconditions: None
// Postcondtions: Posts list of team names to the same channel the command was run
func (b *Bot) teams(discord *discordgo.Session, message *discordgo.MessageCreate) {
	teams, err := b.ApiPtr.GetTeams()
	if err != nil {
		fmt.Println(err)
		discord.ChannelMessageSend(message.ChannelID, "An error occured getting the teams list")
		return
	}
	
	var res strings.Builder
	res.WriteString("Valid teams for this stage are:\n")
	for _, team := range teams {
		res.WriteString(fmt.Sprintf("- %s\n", team))
	}

	discord.ChannelMessageSend(message.ChannelID, res.String())
}

// Function to scrape the upcoming matches, filter for the selected tournament, and post the match details to the discord channel
// Preconditions: Receives UserPrediction struct of a player's predictions
// Postconditions: Sends a message to the discord channel where the command was run containing the upcoming matches
func (b *Bot) upcomingMatches(discord *discordgo.Session, message *discordgo.MessageCreate) {
	matches, err := b.ApiPtr.GetUpcomingMatches()
	if err != nil {
		fmt.Println(err)
		discord.ChannelMessageSend(message.ChannelID, "An error occured getting upcoming matches")
		return
	}
	var res string
	if len(matches) == 0 {
		res = "No upcoming matches"
	} else {
		var builder strings.Builder
		builder.WriteString("Upcoming matches:\n")
		for _,match := range matches {
			if strings.Contains(match, "TBD") {
				continue
			}
			builder.WriteString(match)
		}
		res = builder.String()
	}
	discord.ChannelMessageSend(message.ChannelID, res)
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
