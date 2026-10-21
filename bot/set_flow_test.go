package bot

import (
	"strings"
	"testing"

	"pickems-bot/app"
	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/tournament"

	"github.com/bwmarrin/discordgo"
)

func newSetFlowBot(t *testing.T, kind tournament.Kind, teams []string) (*Bot, *app.MockStore) {
	t.Helper()
	ms := app.NewMockStore(kind, "Stage 1")
	ms.ValidTeams = teams
	ms.SetScheduledMatches([]sources.ScheduledMatch{{Team1: teams[0], Team2: teams[1]}})
	bot, err := NewBot("test_token", app.NewTestApp(ms), nil, "")
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	return bot, ms
}

func makeComponentInteraction(customID string, values ...string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionMessageComponent,
			GuildID:   "test_guild",
			ChannelID: "test_channel",
			Member:    &discordgo.Member{User: &discordgo.User{ID: "test_user", Username: "TestUser"}},
			Data:      discordgo.MessageComponentInteractionData{CustomID: customID, Values: values},
		},
	}
}

// selectMenuIDs returns the custom IDs of every select menu and button in rows.
func selectMenuIDs(rows []discordgo.MessageComponent) []string {
	var ids []string
	for _, r := range rows {
		for _, c := range r.(discordgo.ActionsRow).Components {
			switch v := c.(type) {
			case discordgo.SelectMenu:
				ids = append(ids, v.CustomID)
			case discordgo.Button:
				ids = append(ids, v.CustomID)
			}
		}
	}
	return ids
}

func teamNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = string(rune('A' + i))
	}
	return names
}

func TestSetFlow_Swiss_SavesPredictionFromSelectMenus(t *testing.T) {
	bot, ms := newSetFlowBot(t, tournament.Swiss, teamNames(16))
	session := NewMockDiscordSession()

	bot.newInteractionHandler(session, makeCommandInteraction("set"))

	form := onlyResponse(t, session)
	if form.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("expected the /set form to be ephemeral")
	}
	if got := strings.Join(selectMenuIDs(form.Components), ","); got != "set_select_win,set_select_advance,set_select_lose,set_submit" {
		t.Fatalf("form components = %s", got)
	}

	for _, sel := range []*discordgo.InteractionCreate{
		makeComponentInteraction("set_select_win", "A", "B"),
		makeComponentInteraction("set_select_advance", "C", "D", "E", "F", "G", "H"),
		makeComponentInteraction("set_select_lose", "I", "J"),
	} {
		bot.newInteractionHandler(session, sel)
	}
	for _, r := range session.SentInteractions[1:] {
		if r.Response.Type != discordgo.InteractionResponseDeferredMessageUpdate {
			t.Errorf("expected each selection to be acked with a deferred update, got %v", r.Response.Type)
		}
	}

	bot.newInteractionHandler(session, makeComponentInteraction("set_submit"))

	pred, ok := ms.Predictions["test_user"]
	if !ok {
		t.Fatal("expected the prediction to be saved")
	}
	if strings.Join(pred.Win, ",") != "A,B" || strings.Join(pred.Advance, ",") != "C,D,E,F,G,H" || strings.Join(pred.Lose, ",") != "I,J" {
		t.Errorf("saved buckets = win %v, advance %v, lose %v", pred.Win, pred.Advance, pred.Lose)
	}
	if len(session.Edits) != 1 || session.Edits[0].Embeds == nil || (*session.Edits[0].Embeds)[0].Title != "Pick'Ems Updated" {
		t.Fatalf("expected the deferred submit to be edited with the saved Pick'Ems, got %+v", session.Edits)
	}

	// The session is consumed: a second submit has nothing to save.
	bot.newInteractionHandler(session, makeComponentInteraction("set_submit"))
	if got := session.SentInteractions[len(session.SentInteractions)-1].Response.Data.Content; !strings.Contains(got, "Session expired") {
		t.Errorf("expected a repeat submit to report an expired session, got %q", got)
	}
}

func TestSetFlow_SingleElim_SavesProgression(t *testing.T) {
	bot, ms := newSetFlowBot(t, tournament.SingleElim, teamNames(8))
	session := NewMockDiscordSession()

	bot.newInteractionHandler(session, makeCommandInteraction("set"))
	if got := strings.Join(selectMenuIDs(onlyResponse(t, session).Components), ","); got != "set_select_third,set_select_runnerup,set_select_winner,set_submit" {
		t.Fatalf("form components = %s", got)
	}
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_third", "C", "D"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_runnerup", "B"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_winner", "A"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_submit"))

	want := map[string]models.TeamProgress{
		"A": {Round: "Grand Final", Status: "advanced"},
		"B": {Round: "Grand Final", Status: "eliminated"},
		"C": {Round: "Semi Final", Status: "eliminated"},
		"D": {Round: "Semi Final", Status: "eliminated"},
	}
	got := ms.Predictions["test_user"].Progression
	for team, progress := range want {
		if got[team] != progress {
			t.Errorf("%s: got %+v, want %+v", team, got[team], progress)
		}
	}
}

func TestSetFlow_SameTeamInTwoBuckets_Rejected(t *testing.T) {
	bot, ms := newSetFlowBot(t, tournament.Swiss, teamNames(16))
	session := NewMockDiscordSession()

	bot.newInteractionHandler(session, makeCommandInteraction("set"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_win", "A", "B"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_advance", "A", "D", "E", "F", "G", "H"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_lose", "I", "J"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_submit"))

	last := session.SentInteractions[len(session.SentInteractions)-1].Response.Data
	if !strings.Contains(last.Content, "**A** appears in multiple buckets") || last.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Errorf("expected an ephemeral duplicate-team error, got %+v", last)
	}
	if _, saved := ms.Predictions["test_user"]; saved {
		t.Error("expected nothing to be saved")
	}
}

// Too few picks pass the local duplicate check but fail in the app layer,
// after the response has been deferred.
func TestSetFlow_IncompletePicks_ReportsAppError(t *testing.T) {
	bot, ms := newSetFlowBot(t, tournament.Swiss, teamNames(16))
	session := NewMockDiscordSession()

	bot.newInteractionHandler(session, makeCommandInteraction("set"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_win", "A", "B"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_submit"))

	if len(session.EditedResponses) != 1 || !strings.Contains(session.EditedResponses[0], "incorrect number of teams") {
		t.Errorf("expected the deferred response to explain the error, got %v", session.EditedResponses)
	}
	if _, saved := ms.Predictions["test_user"]; saved {
		t.Error("expected nothing to be saved")
	}
}

func TestSetFlow_SubmitWithoutSet_SessionExpired(t *testing.T) {
	bot, _ := newSetFlowBot(t, tournament.Swiss, teamNames(16))
	session := NewMockDiscordSession()

	bot.newInteractionHandler(session, makeComponentInteraction("set_submit"))

	if got := onlyResponse(t, session).Content; got != "Session expired - please run `/set` again." {
		t.Errorf("content = %q", got)
	}
}

// Selections are tracked per user, so one user's picks never leak into another's.
func TestSetFlow_SelectionsAreScopedPerUser(t *testing.T) {
	bot, ms := newSetFlowBot(t, tournament.Swiss, teamNames(16))
	session := NewMockDiscordSession()

	bot.newInteractionHandler(session, makeCommandInteraction("set"))
	other := makeComponentInteraction("set_select_win", "O", "P")
	other.Member.User = &discordgo.User{ID: "other_user", Username: "Other"}
	bot.newInteractionHandler(session, other)

	bot.newInteractionHandler(session, makeComponentInteraction("set_select_win", "A", "B"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_advance", "C", "D", "E", "F", "G", "H"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_select_lose", "I", "J"))
	bot.newInteractionHandler(session, makeComponentInteraction("set_submit"))

	if got := strings.Join(ms.Predictions["test_user"].Win, ","); got != "A,B" {
		t.Errorf("win picks = %s", got)
	}
}
