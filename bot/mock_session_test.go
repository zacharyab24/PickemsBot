package bot

import "github.com/bwmarrin/discordgo"

// MockDiscordSession records every interaction call a handler makes.
type MockDiscordSession struct {
	SentInteractions []MockInteractionResponse
	// EditedResponses holds the Content of each InteractionResponseEdit.
	EditedResponses []string
	// Edits holds every InteractionResponseEdit payload, embeds included.
	Edits            []*discordgo.WebhookEdit
	DeletedResponses int
	Followups        []*discordgo.WebhookParams
	// ErrorToReturn makes every call fail without recording a result.
	ErrorToReturn error
}

// MockInteractionResponse captures an interaction and its response.
type MockInteractionResponse struct {
	Interaction *discordgo.Interaction
	Response    *discordgo.InteractionResponse
}

func NewMockDiscordSession() *MockDiscordSession {
	return &MockDiscordSession{}
}

func (m *MockDiscordSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse, options ...discordgo.RequestOption) error {
	m.SentInteractions = append(m.SentInteractions, MockInteractionResponse{interaction, resp})
	return m.ErrorToReturn
}

func (m *MockDiscordSession) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}
	m.Edits = append(m.Edits, newresp)
	if newresp.Content != nil {
		m.EditedResponses = append(m.EditedResponses, *newresp.Content)
	}
	return &discordgo.Message{}, nil
}

func (m *MockDiscordSession) InteractionResponseDelete(interaction *discordgo.Interaction, options ...discordgo.RequestOption) error {
	if m.ErrorToReturn != nil {
		return m.ErrorToReturn
	}
	m.DeletedResponses++
	return nil
}

func (m *MockDiscordSession) FollowupMessageCreate(interaction *discordgo.Interaction, wait bool, data *discordgo.WebhookParams, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}
	m.Followups = append(m.Followups, data)
	return &discordgo.Message{}, nil
}
