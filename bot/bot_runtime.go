//go:build !test

/* bot_runtime.go
 * Contains runtime-only Discord bot methods that use *discordgo.Session directly.
 * Delegates to testable handlers in handlers.go to avoid code duplication.
 * Authors: Zachary Bower
 */

package bot

import (
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

// Run starts the Discord bot and listens for messages
func (b *Bot) Run() error {
	// create a session
	discord, err := discordgo.New("Bot " + b.BotToken)
	if err != nil {
		return err
	}

	// add a event handler
	discord.AddHandler(b.newMessage)

	// open session
	discord.Open()
	defer discord.Close() // close session, after function termination

	// keep bot running until there is NO os interruption (ctrl + C)
	log.Println("Pickems Bot started")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	return nil
}

// newMessage delegates to the testable newMessageHandler
// *discordgo.Session implements DiscordSession interface
func (b *Bot) newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	b.newMessageHandler(discord, message, discord.State.User.ID)
}
