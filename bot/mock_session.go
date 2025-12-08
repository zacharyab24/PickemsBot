/* mock_session.go
 * Contains mock implementation of DiscordSession for testing
 * AI-Generated
 */

package bot

import "github.com/bwmarrin/discordgo"

// MockDiscordSession implements DiscordSession for testing purposes
type MockDiscordSession struct {
	// SentMessages stores all messages sent during tests
	SentMessages []MockMessage
	// ErrorToReturn allows tests to simulate errors
	ErrorToReturn error
}

// MockMessage represents a message sent to a channel
type MockMessage struct {
	ChannelID string
	Content   string
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
