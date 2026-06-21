package bot

import (
	"fmt"
	"testing"

	"pickems-bot/app"
	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/store"
	"pickems-bot/tournament"

	"github.com/bwmarrin/discordgo"
)

// region test helpers

func newInteractionTestBot(t *testing.T) (*Bot, *MockDiscordSession) {
	t.Helper()
	mockStore := app.NewMockStore(tournament.SingleElim, "Playoffs")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3"},
	})
	mockStore.SetEliminationResults(map[string]models.TeamProgress{
		"Team A": {Round: "Grand Final", Status: "advanced"},
		"Team B": {Round: "Quarter-finals", Status: "eliminated"},
	})
	mockStore.Predictions["test_user"] = models.Prediction{
		UserID:   "test_user",
		Username: "TestUser",
		Format:   string(tournament.SingleElim),
		Round:    "Playoffs",
		Progression: map[string]models.TeamProgress{
			"Team A": {Round: "Grand Final", Status: "advanced"},
			"Team B": {Round: "Quarter-finals", Status: "eliminated"},
		},
	}
	mockStore.MatchNodes = []sources.MatchNode{
		{Team1: "Team A", Team2: "Team B", Winner: "Team A", Score: "2-1", Section: "Quarterfinal 1: A vs B", Status: "completed"},
		{Team1: "Team C", Team2: "Team D", Winner: "Team C", Score: "2-0", Section: "Quarterfinal 2: C vs D", Status: "completed"},
		{Team1: "Team A", Team2: "Team C", Winner: "Team A", Score: "2-1", Section: "Semifinal 1: A vs C", Status: "completed"},
		{Team1: "Team E", Team2: "Team F", Winner: "Team E", Score: "2-0", Section: "Semifinal 2: E vs F", Status: "completed"},
		{Team1: "Team A", Team2: "Team E", Winner: "", Score: "", Section: "Grand final: A vs E", Status: "pending"},
	}
	mockStore.MatchKind = tournament.SingleElim
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "Team A", Standing: 1},
		{TeamName: "Team B", Standing: 2},
	})
	api := app.NewTestApp(mockStore)
	bot, err := NewBot("test_token", api, nil, "")
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	return bot, NewMockDiscordSession()
}

func makeCommandInteraction(name string, opts ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommand,
			GuildID:   "test_guild",
			ChannelID: "test_channel",
			Member:    &discordgo.Member{User: &discordgo.User{ID: "test_user", Username: "TestUser"}},
			Data:      discordgo.ApplicationCommandInteractionData{Name: name, Options: opts},
		},
	}
}

func makeAutocompleteInteraction(command, optName, typed string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommandAutocomplete,
			GuildID:   "test_guild",
			ChannelID: "test_channel",
			Member:    &discordgo.Member{User: &discordgo.User{ID: "test_user", Username: "TestUser"}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: command,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{Name: optName, Type: discordgo.ApplicationCommandOptionString, Value: typed, Focused: true},
				},
			},
		},
	}
}

// endregion

// region routing tests

func TestInteractionRouting_Leaderboard(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeCommandInteraction("leaderboard"))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
}

func TestInteractionRouting_Upcoming(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeCommandInteraction("upcoming"))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
}

func TestInteractionRouting_Teams(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeCommandInteraction("teams"))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
}

func TestInteractionRouting_Team(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	opt := &discordgo.ApplicationCommandInteractionDataOption{
		Name:  "name",
		Type:  discordgo.ApplicationCommandOptionString,
		Value: "Team A",
	}
	bot.newInteractionHandler(session, makeCommandInteraction("team", opt))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
}

func TestInteractionRouting_Results(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeCommandInteraction("results"))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
}

func TestInteractionRouting_Check(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeCommandInteraction("check"))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
}

func TestInteractionRouting_Autocomplete_DispatchesToAutocompleteHandler(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeAutocompleteInteraction("team", "name", "team"))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
	resp := session.SentInteractions[0].Response
	if resp.Type != discordgo.InteractionApplicationCommandAutocompleteResult {
		t.Errorf("expected autocomplete result type, got %v", resp.Type)
	}
}

func TestInteractionRouting_UnknownCommand_NoResponse(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeCommandInteraction("nonexistent"))
	if len(session.SentInteractions) != 0 {
		t.Errorf("expected no response for unknown command, got %d", len(session.SentInteractions))
	}
}

// endregion

// region autocomplete tests

func TestTeamNameAutocomplete_EmptyInput_ReturnsAllTeams(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeAutocompleteInteraction("team", "name", ""))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) == 0 {
		t.Error("expected choices for empty input, got none")
	}
}

func TestTeamNameAutocomplete_FilterByTyped(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeAutocompleteInteraction("team", "name", "team a"))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) != 1 {
		t.Fatalf("expected 1 choice for 'team a', got %d", len(choices))
	}
	if choices[0].Name != "Team A" {
		t.Errorf("expected 'Team A', got %q", choices[0].Name)
	}
}

func TestTeamNameAutocomplete_NoMatch_ReturnsEmpty(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeAutocompleteInteraction("team", "name", "zzznomatch"))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) != 0 {
		t.Errorf("expected no choices for unmatched input, got %d", len(choices))
	}
}

func TestTeamNameAutocomplete_StoreError_ReturnsEmptyChoices(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	// Inject an error so GetTeams fails
	bot.APIPtr.Store.(*app.MockStore).ListValidTeamsError = fmt.Errorf("teams unavailable")

	bot.newInteractionHandler(session, makeAutocompleteInteraction("team", "name", ""))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) != 0 {
		t.Errorf("expected no choices on store error, got %d", len(choices))
	}
}

// endregion
