/* mock_session.go
 * Contains mock implementation of DiscordSession for testing
 * AI-Generated
 */

package bot

import (
	"io"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// MockDiscordSession implements DiscordSession for testing purposes
type MockDiscordSession struct {
	// SentMessages stores all messages sent during tests (plain and serialised embeds)
	SentMessages []MockMessage
	// SentEmbeds stores all embeds sent via ChannelMessageSendEmbed
	SentEmbeds []MockEmbedMessage
	// SentFiles stores all files sent during tests
	SentFiles []MockFileMessage
	// ErrorToReturn allows tests to simulate errors
	ErrorToReturn error
}

// MockMessage represents a message sent to a channel
type MockMessage struct {
	ChannelID string
	Content   string
}

// MockFileMessage represents a file sent to a channel
type MockFileMessage struct {
	ChannelID string
	Name      string
}

// MockEmbedMessage represents an embed sent to a channel
type MockEmbedMessage struct {
	ChannelID string
	Embed     *discordgo.MessageEmbed
}

// ChannelFileSend implements DiscordSession.ChannelFileSend
func (m *MockDiscordSession) ChannelFileSend(channelID string, name string, r io.Reader, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}
	m.SentFiles = append(m.SentFiles, MockFileMessage{
		ChannelID: channelID,
		Name:      name,
	})
	return &discordgo.Message{ID: "mock_message_id", ChannelID: channelID}, nil
}

// ChannelMessageSend implements DiscordSession.ChannelMessageSend
func (m *MockDiscordSession) ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}

	m.SentMessages = append(m.SentMessages, MockMessage{
		ChannelID: channelID,
		Content:   content,
	})

	return &discordgo.Message{
		ID:        "mock_message_id",
		ChannelID: channelID,
		Content:   content,
	}, nil
}

// ChannelMessageSendEmbed implements DiscordSession.ChannelMessageSendEmbed.
// It stores the embed in SentEmbeds and also appends a serialised form to
// SentMessages so that routing tests (Len checks) keep working without change.
func (m *MockDiscordSession) ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}
	m.SentEmbeds = append(m.SentEmbeds, MockEmbedMessage{ChannelID: channelID, Embed: embed})

	// Serialise embed content so existing msg.Content string assertions still work
	var sb strings.Builder
	if embed.Title != "" {
		sb.WriteString(embed.Title + "\n")
	}
	if embed.Description != "" {
		sb.WriteString(embed.Description + "\n")
	}
	for _, f := range embed.Fields {
		sb.WriteString(f.Name + "\n" + f.Value + "\n")
	}
	m.SentMessages = append(m.SentMessages, MockMessage{ChannelID: channelID, Content: sb.String()})
	return &discordgo.Message{ID: "mock_message_id", ChannelID: channelID}, nil
}

// GetLastEmbed returns the last embed sent, or nil if none
func (m *MockDiscordSession) GetLastEmbed() *MockEmbedMessage {
	if len(m.SentEmbeds) == 0 {
		return nil
	}
	return &m.SentEmbeds[len(m.SentEmbeds)-1]
}

// GetLastMessage returns the last message sent, or empty MockMessage if none
func (m *MockDiscordSession) GetLastMessage() MockMessage {
	if len(m.SentMessages) == 0 {
		return MockMessage{}
	}
	return m.SentMessages[len(m.SentMessages)-1]
}

// ClearMessages clears all stored messages
func (m *MockDiscordSession) ClearMessages() {
	m.SentMessages = nil
}

// NewMockDiscordSession creates a new MockDiscordSession for testing
func NewMockDiscordSession() *MockDiscordSession {
	return &MockDiscordSession{
		SentMessages: make([]MockMessage, 0),
	}
}
