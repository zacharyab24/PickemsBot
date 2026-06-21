/* session_interface.go
 * Contains interface for Discord session to enable mocking in tests
 * AI-Generated
 */

package bot

import (
	"io"

	"github.com/bwmarrin/discordgo"
)

// DiscordSession defines the Discord session methods used by the bot.
// This interface allows for easy mocking in tests.
type DiscordSession interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelFileSend(channelID string, name string, r io.Reader, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
	InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse, options ...discordgo.RequestOption) error
	InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

// Ensure *discordgo.Session implements DiscordSession
var _ DiscordSession = (*discordgo.Session)(nil)
