/* session_interface.go
 * Contains interface for Discord session to enable mocking in tests
 * AI-Generated
 */

package bot

import "github.com/bwmarrin/discordgo"

// DiscordSession defines the Discord session methods used by the bot.
// This interface allows for easy mocking in tests.
type DiscordSession interface {
	InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse, options ...discordgo.RequestOption) error
	InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
	InteractionResponseDelete(interaction *discordgo.Interaction, options ...discordgo.RequestOption) error
	FollowupMessageCreate(interaction *discordgo.Interaction, wait bool, data *discordgo.WebhookParams, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

// Ensure *discordgo.Session implements DiscordSession
var _ DiscordSession = (*discordgo.Session)(nil)
