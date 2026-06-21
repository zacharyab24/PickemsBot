//go:build !test

/* bot_runtime.go
 * Contains runtime-only Discord bot methods that use *discordgo.Session directly.
 * Delegates to testable handlers in handlers.go to avoid code duplication.
 * Authors: Zachary Bower
 */

package bot

import (
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

	// open session
	discord.Open()
	b.session = discord
	defer discord.Close() // close session, after function termination

	// register handlers for slash commands and component interactions
	b.registerSlashCommands(discord)
	discord.AddHandler(b.newInteraction)

	// keep bot running until there is NO os interruption (ctrl + C)
	b.logger().Info("Pickems Bot started")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	return nil
}

func (b *Bot) newInteraction(discord *discordgo.Session, i *discordgo.InteractionCreate) {
	b.newInteractionHandler(discord, i)
}

// registerSlashCommands registers the bot's slash commands with Discord
func (b *Bot) registerSlashCommands(discord *discordgo.Session) {
	discord.ApplicationCommandCreate(discord.State.User.ID, b.devGuildID, &discordgo.ApplicationCommand{
		Name:        "check",
		Description: "See your currently saved Pick'ems",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "User to check (defaults to yourself)",
				Required:    false,
			},
		},
	})

	discord.ApplicationCommandCreate(discord.State.User.ID, b.devGuildID, &discordgo.ApplicationCommand{
		Name:        "set",
		Description: "Set your Pick'ems",
	})

	discord.ApplicationCommandCreate(discord.State.User.ID, b.devGuildID, &discordgo.ApplicationCommand{
		Name:        "teams",
		Description: "See teams in the current round of the tournament",
	})

	discord.ApplicationCommandCreate(discord.State.User.ID, b.devGuildID, &discordgo.ApplicationCommand{
		Name:        "team",
		Description: "See specific info about a team in the tournament",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "name",
				Autocomplete: true,
				Description:  "Team name",
				Required:     true,
			},
		},
	})

	discord.ApplicationCommandCreate(discord.State.User.ID, b.devGuildID, &discordgo.ApplicationCommand{
		Name:        "leaderboard",
		Description: "See who has the most correct picks in this stage of the tournament",
	})

	discord.ApplicationCommandCreate(discord.State.User.ID, b.devGuildID, &discordgo.ApplicationCommand{
		Name:        "upcoming",
		Description: "See the upcoming matches in the tournament",
	})

	discord.ApplicationCommandCreate(discord.State.User.ID, b.devGuildID, &discordgo.ApplicationCommand{
		Name:        "results",
		Description: "See the results of completed matches in the tournament",
	})
}
