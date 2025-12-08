/* handlers.go
 * Contains testable handler methods that accept DiscordSession interface
 * Authors: Zachary Bower
 * AI-Generated: Extracted runtime functionality from bot.go
 */

package bot

import (
	"errors"
	"fmt"
	"log"
	"pickems-bot/api/shared"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/go-andiamo/splitter"
	"go.mongodb.org/mongo-driver/mongo"
)

// helpMessageHandler handles the $help command with a DiscordSession interface
func (b *Bot) helpMessageHandler(session DiscordSession, message *discordgo.MessageCreate) {
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
	session.ChannelMessageSend(message.ChannelID, res.String())
}

// detailsHandler handles the $details command with a DiscordSession interface
func (b *Bot) detailsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	info, err := b.APIPtr.GetTournamentInfo()
	if err != nil {
		fmt.Println(err)
		session.ChannelMessageSend(message.ChannelID, "An unexpected error occured")
		return
	}
	var res strings.Builder
	for i := range info {
		res.WriteString(fmt.Sprintf("%s\n", info[i]))
	}
	session.ChannelMessageSend(message.ChannelID, res.String())
}

// setPredictionsHandler handles the $set command with a DiscordSession interface
func (b *Bot) setPredictionsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	user := shared.User{UserID: message.Author.ID, Username: message.Author.Username}
	res := fmt.Sprintf("%s's Pickems have been updated\n", user.Username)

	// Get User Predictions from message
	spaceSplitter, _ := splitter.NewSplitter(' ', splitter.DoubleQuotes, splitter.LeftRightDoubleDoubleQuotes)
	msg, _ := spaceSplitter.Split(message.Content)
	userPreds := msg[1:]

	err := b.APIPtr.SetUserPrediction(user, userPreds, b.APIPtr.Store.GetRound())
	if err != nil {
		fmt.Println(err)
		res = fmt.Sprintf("An error occured setting %s's Pickems: %s", user.Username, err)
	}
	session.ChannelMessageSend(message.ChannelID, res)
}

// checkPredictionsHandler handles the $check command with a DiscordSession interface
func (b *Bot) checkPredictionsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	user := shared.User{UserID: message.Author.ID, Username: message.Author.Username}
	res, err := b.APIPtr.CheckPrediction(user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			res = fmt.Sprintf("%s does not have any Pickems stored. Use $set to set your predictions\n", user.Username)
		} else {
			log.Println(err)
			res = fmt.Sprintf("An error occured checking %s's Pickems", user.Username)
		}
	}
	session.ChannelMessageSend(message.ChannelID, res)
}

// leaderboardHandler handles the $leaderboard command with a DiscordSession interface
func (b *Bot) leaderboardHandler(session DiscordSession, message *discordgo.MessageCreate) {
	res, err := b.APIPtr.GetLeaderboard()
	if err != nil {
		fmt.Println(err)
		res = "An error occurred getting the leaderboard"
	}
	session.ChannelMessageSend(message.ChannelID, res)
}

// teamsHandler handles the $teams command with a DiscordSession interface
func (b *Bot) teamsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	teams, err := b.APIPtr.GetTeams()
	if err != nil {
		fmt.Println(err)
		session.ChannelMessageSend(message.ChannelID, "An error occured getting the teams list")
		return
	}

	var res strings.Builder
	res.WriteString("Valid teams for this stage are:\n")
	for _, team := range teams {
		res.WriteString(fmt.Sprintf("- %s\n", team))
	}

	session.ChannelMessageSend(message.ChannelID, res.String())
}

// upcomingMatchesHandler handles the $upcoming command with a DiscordSession interface
func (b *Bot) upcomingMatchesHandler(session DiscordSession, message *discordgo.MessageCreate) {
	matches, err := b.APIPtr.GetUpcomingMatches()
	if err != nil {
		fmt.Println(err)
		session.ChannelMessageSend(message.ChannelID, "An error occured getting upcoming matches")
		return
	}
	var res strings.Builder
	if len(matches) == 0 {
		res.WriteString("No upcoming matches")
	} else {
		res.WriteString("Upcoming matches:\n")
		for _, match := range matches {
			if strings.Contains(match, "TBD") {
				continue
			}
			res.WriteString(match)
		}
	}
	session.ChannelMessageSend(message.ChannelID, res.String())
}

// newMessageHandler routes messages to appropriate handlers with a DiscordSession interface
// botUserID is the bot's user ID to prevent self-responses
func (b *Bot) newMessageHandler(session DiscordSession, message *discordgo.MessageCreate, botUserID string) {
	// Prevent bot from responding to its own messages
	if message.Author.ID == botUserID {
		return
	}

	// Route to appropriate handler
	switch {
	case startsWith(message.Content, "$help"):
		b.helpMessageHandler(session, message)

	case startsWith(message.Content, "$details"):
		b.detailsHandler(session, message)

	case startsWith(message.Content, "$set"):
		b.setPredictionsHandler(session, message)

	case startsWith(message.Content, "$check"):
		b.checkPredictionsHandler(session, message)

	case startsWith(message.Content, "$leaderboard"):
		b.leaderboardHandler(session, message)

	case startsWith(message.Content, "$teams"):
		b.teamsHandler(session, message)

	case startsWith(message.Content, "$upcoming"):
		b.upcomingMatchesHandler(session, message)
	}
}
