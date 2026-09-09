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

// makeConfigAutocomplete builds an autocomplete interaction for /config. sub is
// the subcommand name, focused is the option being completed, and opts are the
// option values already present on the interaction (including the focused one's
// partial text).
func makeConfigAutocomplete(sub, focused string, opts ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	for _, o := range opts {
		if o.Name == focused {
			o.Focused = true
		}
	}
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommandAutocomplete,
			GuildID:   "test_guild",
			ChannelID: "test_channel",
			Member:    &discordgo.Member{User: &discordgo.User{ID: "admin_user"}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name:    "config",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{configSub(sub, opts...)},
			},
		},
	}
}

// lastContent returns the meaningful final message content, whether the
// handler responded in one shot (InteractionRespond) or deferred first and
// finalised via InteractionResponseEdit (see deferEphemeral) - in the latter
// case the deferred ack itself carries no content, so the edit is what matters.
func lastContent(t *testing.T, session *MockDiscordSession) string {
	t.Helper()
	if len(session.EditedResponses) > 0 {
		if len(session.EditedResponses) != 1 {
			t.Fatalf("expected exactly 1 edited response, got %d: %v", len(session.EditedResponses), session.EditedResponses)
		}
		return session.EditedResponses[0]
	}
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

// seedBlast wires the mock so the channel is configured on the BLAST Qualifier
// row (id 1) and both BLAST rounds are resolvable - the state /config set-round
// operates from.
func seedBlast(bot *Bot) {
	ms := bot.APIPtr.Store.(*app.MockStore)
	tid := 1
	ms.GuildConfig = store.GuildConfig{GuildID: "test_guild", TournamentID: &tid}
	ms.Tournaments = []store.Tournament{
		{ID: 1, Name: "BLAST Bounty Summer 2026", Round: "Qualifier"},
		{ID: 2, Name: "BLAST Bounty Summer 2026", Round: "Playoffs"},
	}
}

func TestConfigSetRound_Success(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	seedBlast(bot)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-round", strArg("round", "Playoffs"))))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	if session.SentInteractions[0].Response.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("expected ephemeral response")
	}
	// Switching round repoints tournament_id at the sibling row and stores the round.
	ms := bot.APIPtr.Store.(*app.MockStore)
	if ms.UpsertedGuildConfig == nil || ms.UpsertedGuildConfig.Round == nil || ms.UpsertedGuildConfig.TournamentID == nil {
		t.Fatal("expected round and tournament id to be persisted")
	}
	if *ms.UpsertedGuildConfig.Round != "Playoffs" {
		t.Errorf("expected round 'Playoffs', got %q", *ms.UpsertedGuildConfig.Round)
	}
	if *ms.UpsertedGuildConfig.TournamentID != 2 {
		t.Errorf("expected tournament id 2 (Playoffs row), got %d", *ms.UpsertedGuildConfig.TournamentID)
	}
}

// TestConfigSetRound_DefersBeforeResponding verifies the handler defers
// immediately, before any Store work, instead of a single-shot response that
// could arrive after Discord's 3s window.
func TestConfigSetRound_DefersBeforeResponding(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	seedBlast(bot)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-round", strArg("round", "Playoffs"))))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected exactly 1 initial response (the deferred ack), got %d", len(session.SentInteractions))
	}
	resp := session.SentInteractions[0].Response
	if resp.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
		t.Errorf("expected a deferred ack, got response type %v", resp.Type)
	}
	if resp.Data == nil || resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("expected the deferred ack to be ephemeral")
	}
	if len(session.EditedResponses) != 1 {
		t.Fatalf("expected exactly 1 edit finalising the response, got %d", len(session.EditedResponses))
	}
}

// A round that doesn't belong to the configured tournament can't be resolved to a row.
func TestConfigSetRound_UnknownRound_Rejected(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	seedBlast(bot)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-round", strArg("round", "Group Stage"))))

	if !strings.Contains(lastContent(t, session), "Could not set that round") {
		t.Errorf("expected round error, got %q", lastContent(t, session))
	}
}

func TestConfigSetRound_StoreError(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	seedBlast(bot)
	bot.APIPtr.Store.(*app.MockStore).UpsertGuildConfigError = fmt.Errorf("write failed")

	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms, configSub("set-round", strArg("round", "Playoffs"))))

	if !strings.Contains(lastContent(t, session), "Could not set that round") {
		t.Errorf("expected round-update error, got %q", lastContent(t, session))
	}
}

// endregion

// region set-tournament

func TestConfigSetTournament_Success(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	seedBlast(bot)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms,
		configSub("set-tournament", strArg("tournament", "BLAST Bounty Summer 2026"), strArg("round", "Playoffs"))))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	// The (name, round) pair resolves to the Playoffs row (id 2), stored with its round.
	ms := bot.APIPtr.Store.(*app.MockStore)
	if ms.UpsertedGuildConfig == nil || ms.UpsertedGuildConfig.TournamentID == nil || ms.UpsertedGuildConfig.Round == nil {
		t.Fatal("expected tournament id and round to be persisted")
	}
	if *ms.UpsertedGuildConfig.TournamentID != 2 {
		t.Errorf("expected tournament id 2, got %d", *ms.UpsertedGuildConfig.TournamentID)
	}
	if *ms.UpsertedGuildConfig.Round != "Playoffs" {
		t.Errorf("expected round 'Playoffs', got %q", *ms.UpsertedGuildConfig.Round)
	}
}

// A user can free-type an autocomplete option, so a (name, round) pair matching
// no row must be rejected before anything is persisted.
func TestConfigSetTournament_UnknownPair_Rejected(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	seedBlast(bot)
	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms,
		configSub("set-tournament", strArg("tournament", "Nonexistent Cup"), strArg("round", "Playoffs"))))

	if !strings.Contains(lastContent(t, session), "Could not set that tournament") {
		t.Errorf("expected pick-from-list message, got %q", lastContent(t, session))
	}
}

func TestConfigSetTournament_StoreError(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	seedBlast(bot)
	bot.APIPtr.Store.(*app.MockStore).UpsertGuildConfigError = fmt.Errorf("fk violation")

	bot.newInteractionHandler(session, makeConfigInteraction(adminPerms,
		configSub("set-tournament", strArg("tournament", "BLAST Bounty Summer 2026"), strArg("round", "Playoffs"))))

	if !strings.Contains(lastContent(t, session), "Could not set that tournament") {
		t.Errorf("expected set-tournament error, got %q", lastContent(t, session))
	}
}

// endregion

// region autocomplete

func TestConfigTournamentAutocomplete_ReturnsDistinctNames(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	// Two rows share a name (different rounds) - the picklist must collapse them.
	bot.APIPtr.Store.(*app.MockStore).Tournaments = []store.Tournament{
		{ID: 1, Name: "Alpha Major", Round: "Qualifier"},
		{ID: 2, Name: "Alpha Major", Round: "Playoffs"},
		{ID: 3, Name: "Beta Cup", Round: "Playoffs"},
	}

	bot.newInteractionHandler(session, makeConfigAutocomplete("set-tournament", "tournament", strArg("tournament", "")))

	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 response, got %d", len(session.SentInteractions))
	}
	resp := session.SentInteractions[0].Response
	if resp.Type != discordgo.InteractionApplicationCommandAutocompleteResult {
		t.Fatalf("expected autocomplete result type, got %v", resp.Type)
	}
	choices := resp.Data.Choices
	if len(choices) != 2 {
		t.Fatalf("expected 2 distinct-name choices, got %d", len(choices))
	}
	// Value is the name - that's what set-tournament resolves back.
	if choices[0].Name != "Alpha Major" || choices[0].Value != "Alpha Major" {
		t.Errorf("expected Alpha Major=>Alpha Major, got %s=>%v", choices[0].Name, choices[0].Value)
	}
}

func TestConfigTournamentAutocomplete_FilterByTyped(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).Tournaments = []store.Tournament{
		{ID: 1, Name: "Alpha Major", Round: "Playoffs"},
		{ID: 2, Name: "Beta Cup", Round: "Playoffs"},
	}

	bot.newInteractionHandler(session, makeConfigAutocomplete("set-tournament", "tournament", strArg("tournament", "beta")))

	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) != 1 || choices[0].Name != "Beta Cup" {
		t.Fatalf("expected only Beta Cup, got %+v", choices)
	}
}

// The round option on set-tournament is scoped to the sibling tournament value.
func TestConfigRoundAutocomplete_ScopedToTournament(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).Tournaments = []store.Tournament{
		{ID: 1, Name: "Alpha Major", Round: "Qualifier"},
		{ID: 2, Name: "Alpha Major", Round: "Playoffs"},
		{ID: 3, Name: "Beta Cup", Round: "Group Stage"},
	}

	bot.newInteractionHandler(session, makeConfigAutocomplete("set-tournament", "round",
		strArg("tournament", "Alpha Major"), strArg("round", "")))

	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) != 2 {
		t.Fatalf("expected 2 rounds for Alpha Major, got %d: %+v", len(choices), choices)
	}
}

// Completing the round before a tournament is chosen yields no suggestions.
func TestConfigRoundAutocomplete_NoTournamentYet_Empty(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).Tournaments = []store.Tournament{
		{ID: 1, Name: "Alpha Major", Round: "Qualifier"},
	}

	bot.newInteractionHandler(session, makeConfigAutocomplete("set-tournament", "round",
		strArg("tournament", ""), strArg("round", "")))

	if choices := session.SentInteractions[0].Response.Data.Choices; len(choices) != 0 {
		t.Errorf("expected no choices before a tournament is picked, got %d", len(choices))
	}
}

// set-round scopes its round option to the already-configured tournament.
func TestConfigSetRoundAutocomplete_ScopedToConfigured(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	seedBlast(bot) // configured on BLAST (id 1), which has Qualifier + Playoffs

	bot.newInteractionHandler(session, makeConfigAutocomplete("set-round", "round", strArg("round", "")))

	if choices := session.SentInteractions[0].Response.Data.Choices; len(choices) != 2 {
		t.Fatalf("expected 2 rounds for configured tournament, got %d: %+v", len(choices), choices)
	}
}

func TestConfigTournamentAutocomplete_StoreError_ReturnsEmpty(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).ListTournamentNamesError = fmt.Errorf("db down")

	bot.newInteractionHandler(session, makeConfigAutocomplete("set-tournament", "tournament", strArg("tournament", "")))

	choices := session.SentInteractions[0].Response.Data.Choices
	if len(choices) != 0 {
		t.Errorf("expected no choices on store error, got %d", len(choices))
	}
}

// endregion
