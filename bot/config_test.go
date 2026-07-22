package bot

import (
	"fmt"
	"strings"
	"testing"

	"pickems-bot/app"
	"pickems-bot/store"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
)

// region /config test helpers

const adminPerms = int64(discordgo.PermissionManageGuild)

// configSub wraps args in a subcommand option, mirroring how Discord nests a
// subcommand's arguments one level below the command.
func configSub(name string, args ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:    name,
		Type:    discordgo.ApplicationCommandOptionSubCommand,
		Options: args,
	}
}

func strArg(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

func makeConfigInteraction(perms int64, sub *discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommand,
			GuildID:   "test_guild",
			ChannelID: "test_channel",
			Member: &discordgo.Member{
				User:        &discordgo.User{ID: "admin_user", Username: "Admin"},
				Permissions: perms,
			},
			Data: discordgo.ApplicationCommandInteractionData{
				Name:    "config",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{sub},
			},
		},
	}
}

// makeConfigAutocomplete builds an autocomplete interaction whose focused option
// is nested under the set-tournament subcommand.
func makeConfigAutocomplete(typed string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommandAutocomplete,
			GuildID:   "test_guild",
			ChannelID: "test_channel",
			Member:    &discordgo.Member{User: &discordgo.User{ID: "admin_user"}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "config",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					configSub("set-tournament", &discordgo.ApplicationCommandInteractionDataOption{
						Name:    "tournament",
						Type:    discordgo.ApplicationCommandOptionString,
						Value:   typed,
						Focused: true,
					}),
				},
			},
		},
	}
}

func lastContent(t *testing.T, session *MockDiscordSession) string {
	t.Helper()
	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
	return session.SentInteractions[0].Response.Data.Content
}

// endregion

// region routing + permission gate

func TestInteractionRouting_Config(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("view")))
	if len(session.SentInteractions) != 1 {
		t.Errorf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
}

func TestConfig_NonAdmin_Rejected(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeConfigInteraction(0, configSub("view")))

	content := lastContent(t, session)
	if !strings.Contains(content, "Manage Server") {
		t.Errorf("expected permission-denied message, got %q", content)
	}
	if session.SentInteractions[0].Response.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("expected ephemeral response")
	}
}

func TestConfig_NilMember_Rejected(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	i := makeConfigInteraction(adminPerms, configSub("view"))
	i.Member = nil
	bot.newInteractionHandler(session, i)

	if !strings.Contains(lastContent(t, session), "server") {
		t.Errorf("expected server-only message, got %q", lastContent(t, session))
	}
}

// endregion

// region view

func TestConfigView_ShowsConfig(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	name := "IEM Cologne 2026"
	round := "Playoffs"
	bot.APIPtr.Store.(*app.MockStore).GuildConfig = store.GuildConfig{
		GuildID:        "test_guild",
		TournamentName: &name,
		Round:          &round,
	}

	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("view")))

	content := lastContent(t, session)
	if !strings.Contains(content, name) || !strings.Contains(content, round) {
		t.Errorf("expected tournament and round in view, got %q", content)
	}
}

func TestConfigView_NotConfigured(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).GetGuildConfigError = pgx.ErrNoRows

	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("view")))

	if !strings.Contains(lastContent(t, session), "No configuration set") {
		t.Errorf("expected not-configured message, got %q", lastContent(t, session))
	}
}

func TestConfigView_StoreError(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).GetGuildConfigError = fmt.Errorf("db down")

	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("view")))

	if !strings.Contains(lastContent(t, session), "error occurred loading") {
		t.Errorf("expected load-error message, got %q", lastContent(t, session))
	}
}

// endregion

// region set-round

func TestConfigSetRound_Success(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-round", strArg("round", "Grand Final"))))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	if session.SentInteractions[0].Response.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("expected ephemeral response")
	}
	// Assert on the persisted effect, not the confirmation wording.
	ms := bot.APIPtr.Store.(*app.MockStore)
	if ms.UpsertedGuildConfig == nil || ms.UpsertedGuildConfig.Round == nil {
		t.Fatal("expected round to be persisted")
	}
	if *ms.UpsertedGuildConfig.Round != "Grand Final" {
		t.Errorf("expected round 'Grand Final', got %q", *ms.UpsertedGuildConfig.Round)
	}
}

func TestConfigSetRound_StoreError(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).UpsertGuildConfigError = fmt.Errorf("write failed")

	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-round", strArg("round", "Playoffs"))))

	if !strings.Contains(lastContent(t, session), "error occurred updating the round") {
		t.Errorf("expected round-update error, got %q", lastContent(t, session))
	}
}

// endregion

// region set-tournament

func TestConfigSetTournament_Success(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-tournament", strArg("tournament", "5"))))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	// Assert the parsed DB id reached the store, independent of confirmation wording.
	ms := bot.APIPtr.Store.(*app.MockStore)
	if ms.UpsertedGuildConfig == nil || ms.UpsertedGuildConfig.TournamentID == nil {
		t.Fatal("expected tournament id to be persisted")
	}
	if *ms.UpsertedGuildConfig.TournamentID != 5 {
		t.Errorf("expected tournament id 5, got %d", *ms.UpsertedGuildConfig.TournamentID)
	}
}

// A user can submit free text for an autocomplete option, so a non-numeric value
// must be rejected before it reaches the store.
func TestConfigSetTournament_NonNumeric_Rejected(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-tournament", strArg("tournament", "not-an-id"))))

	if !strings.Contains(lastContent(t, session), "pick a tournament from the list") {
		t.Errorf("expected pick-from-list message, got %q", lastContent(t, session))
	}
}

func TestConfigSetTournament_StoreError(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).UpsertGuildConfigError = fmt.Errorf("fk violation")

	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-tournament", strArg("tournament", "5"))))

	if !strings.Contains(lastContent(t, session), "Could not set that tournament") {
		t.Errorf("expected set-tournament error, got %q", lastContent(t, session))
	}
}

// endregion

// region autocomplete

func TestConfigTournamentAutocomplete_ReturnsChoices(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).Tournaments = []store.Tournament{
		{ID: 1, Name: "Alpha Major"},
		{ID: 2, Name: "Beta Cup"},
	}

	bot.newInteractionHandler(session, makeConfigAutocomplete(""))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	resp := session.SentInteractions[0].Response
	if resp.Type != discordgo.InteractionApplicationCommandAutocompleteResult {
		t.Fatalf("expected autocomplete result type, got %v", resp.Type)
	}
	choices := resp.Data.Choices
	if len(choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(choices))
	}
	// Value is the DB id as a string — that's what set-tournament parses back.
	if choices[0].Name != "Alpha Major" || choices[0].Value != "1" {
		t.Errorf("expected Alpha Major=>1, got %s=>%v", choices[0].Name, choices[0].Value)
	}
}

func TestConfigTournamentAutocomplete_FilterByTyped(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).Tournaments = []store.Tournament{
		{ID: 1, Name: "Alpha Major"},
		{ID: 2, Name: "Beta Cup"},
	}

	bot.newInteractionHandler(session, makeConfigAutocomplete("beta"))

	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) != 1 || choices[0].Name != "Beta Cup" {
		t.Fatalf("expected only Beta Cup, got %+v", choices)
	}
}

func TestConfigTournamentAutocomplete_StoreError_ReturnsEmpty(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).ListTournamentsError = fmt.Errorf("db down")

	bot.newInteractionHandler(session, makeConfigAutocomplete(""))

	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) != 0 {
		t.Errorf("expected no choices on store error, got %d", len(choices))
	}
}

// endregion
