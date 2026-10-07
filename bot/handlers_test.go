package bot

import (
	"fmt"
	"strings"
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

// TestResultsInteractionHandler_UnknownFormat_UsesChronologicalFallback verifies formats
// without a dedicated renderer (e.g. double-elimination) get the chronological fallback
// instead of the old "Unsupported tournament format." message.
func TestResultsInteractionHandler_UnknownFormat_UsesChronologicalFallback(t *testing.T) {
	mockStore := app.NewMockStore(tournament.DoubleElim, "Playoffs")
	mockStore.MatchKind = tournament.DoubleElim
	mockStore.MatchNodes = []sources.MatchNode{
		{Team1: "Team A", Team2: "Team B", Winner: "Team A", Score: "2-1", Status: "completed"},
		{Team1: "Team C", Team2: "Team D", Status: "pending"},
	}
	api := app.NewTestApp(mockStore)
	bot, err := NewBot("test_token", api, nil, "")
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	session := NewMockDiscordSession()

	bot.resultsInteractionHandler(session, makeCommandInteraction("results"))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
	data := session.SentInteractions[0].Response.Data
	if data.Flags&discordgo.MessageFlagsIsComponentsV2 == 0 {
		t.Error("expected ComponentsV2 response, not the unsupported-format message")
	}
	if len(data.Components) != 1 {
		t.Errorf("expected 1 container component, got %d", len(data.Components))
	}
}

func TestInteractionRouting_Check(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeCommandInteraction("check"))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
}

// region prediction commands gated on format

// A guild config pointing at an unsupported-for-predictions format (e.g.
// double-elimination) should surface the same friendly message from every
// prediction command, not a generic error - see app.ErrFormatDoesNotSupportPredictions.
func newUnsupportedFormatTestBot(t *testing.T) (*Bot, *MockDiscordSession) {
	t.Helper()
	mockStore := app.NewMockStore(tournament.DoubleElim, "Playoffs")
	format := string(tournament.DoubleElim)
	mockStore.GuildConfig.Format = &format
	api := app.NewTestApp(mockStore)
	bot, err := NewBot("test_token", api, nil, "")
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	return bot, NewMockDiscordSession()
}

func TestCheckInteractionHandler_UnsupportedFormat_ShowsFriendlyMessage(t *testing.T) {
	bot, session := newUnsupportedFormatTestBot(t)

	bot.checkInteractionHandler(session, makeCommandInteraction("check"))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
	content := session.SentInteractions[0].Response.Data.Content
	if !strings.Contains(content, "Predictions are not supported for this tournament format") {
		t.Errorf("expected friendly format-not-supported message, got %q", content)
	}
}

func TestLeaderboardInteractionHandler_UnsupportedFormat_ShowsFriendlyMessage(t *testing.T) {
	bot, session := newUnsupportedFormatTestBot(t)

	bot.leaderboardInteractionHandler(session, makeCommandInteraction("leaderboard"))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
	content := session.SentInteractions[0].Response.Data.Content
	if !strings.Contains(content, "Predictions are not supported for this tournament format") {
		t.Errorf("expected friendly format-not-supported message, got %q", content)
	}
}

// setInteractionHandler is the real gate for /set - it resolves the format
// itself (via GetTournamentInfo) before ever showing team-selection UI, so
// this exercises the actual entry point rather than App.SetUserPrediction's
// defensive backstop.
func TestSetInteractionHandler_UnsupportedFormat_ShowsFriendlyMessage(t *testing.T) {
	mockStore := app.NewMockStore(tournament.DoubleElim, "Playoffs")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	api := app.NewTestApp(mockStore)
	bot, err := NewBot("test_token", api, nil, "")
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	session := NewMockDiscordSession()

	bot.setInteractionHandler(session, makeCommandInteraction("set"))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
	content := session.SentInteractions[0].Response.Data.Content
	if !strings.Contains(content, "Predictions are not supported for this tournament format (double-elimination)") {
		t.Errorf("expected friendly format-not-supported message naming the format, got %q", content)
	}
}

// endregion

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
