package bot

import (
	"strings"
	"testing"

	"pickems-bot/app"
	"pickems-bot/store"
	"pickems-bot/tournament"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
)

// containerText joins every TextDisplay inside a Components V2 response.
func containerText(t *testing.T, data *discordgo.InteractionResponseData) string {
	t.Helper()
	if data.Flags&discordgo.MessageFlagsIsComponentsV2 == 0 {
		t.Fatalf("expected a Components V2 response, got content %q", data.Content)
	}
	var sb strings.Builder
	var walk func([]discordgo.MessageComponent)
	walk = func(comps []discordgo.MessageComponent) {
		for _, c := range comps {
			switch v := c.(type) {
			case discordgo.Container:
				walk(v.Components)
			case discordgo.Section:
				walk(v.Components)
			case discordgo.TextDisplay:
				sb.WriteString(v.Content + "\n")
			}
		}
	}
	walk(data.Components)
	return sb.String()
}

// onlyResponse returns the single InteractionRespond payload a handler sent.
func onlyResponse(t *testing.T, session *MockDiscordSession) *discordgo.InteractionResponseData {
	t.Helper()
	if len(session.SentInteractions) != 1 {
		t.Fatalf("expected 1 interaction response, got %d", len(session.SentInteractions))
	}
	return session.SentInteractions[0].Response.Data
}

func onlyEmbed(t *testing.T, session *MockDiscordSession) *discordgo.MessageEmbed {
	t.Helper()
	data := onlyResponse(t, session)
	if len(data.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d (content %q)", len(data.Embeds), data.Content)
	}
	return data.Embeds[0]
}

func TestCheckCommand_ShowsCallersScore(t *testing.T) {
	bot, session := newInteractionTestBot(t)

	bot.newInteractionHandler(session, makeCommandInteraction("check"))

	embed := onlyEmbed(t, session)
	if embed.Title != "TestUser's Pick'Ems" {
		t.Errorf("title = %q", embed.Title)
	}
	if !strings.HasPrefix(embed.Description, "**2/") {
		t.Errorf("expected both picks to be scored correct, got %q", embed.Description)
	}
	if len(embed.Fields) != 1 || !strings.Contains(embed.Fields[0].Value, "🏆 Champion: **Team A** ✅") {
		t.Errorf("expected Team A shown as a correct champion pick, got %+v", embed.Fields)
	}
}

func TestCheckCommand_OtherUser_ResolvesMentionedUser(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	i := makeCommandInteraction("check", &discordgo.ApplicationCommandInteractionDataOption{
		Name: "user", Type: discordgo.ApplicationCommandOptionUser, Value: "test_user",
	})
	i.Data = discordgo.ApplicationCommandInteractionData{
		Name:    "check",
		Options: i.ApplicationCommandData().Options,
		Resolved: &discordgo.ApplicationCommandInteractionDataResolved{
			Users: map[string]*discordgo.User{"test_user": {ID: "test_user", Username: "testuser", GlobalName: "Display Name"}},
		},
	}

	bot.newInteractionHandler(session, i)

	if got := onlyEmbed(t, session).Title; got != "Display Name's Pick'Ems" {
		t.Errorf("expected the mentioned user's display name, got %q", got)
	}
}

func TestCheckCommand_NoPrediction_TellsUserToSet(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	delete(bot.APIPtr.Store.(*app.MockStore).Predictions, "test_user")
	bot.APIPtr.Store.(*app.MockStore).GetPredictionError = pgx.ErrNoRows

	bot.newInteractionHandler(session, makeCommandInteraction("check"))

	if got := onlyResponse(t, session).Content; !strings.Contains(got, "Use `/set`") {
		t.Errorf("expected a prompt to run /set, got %q", got)
	}
}

func TestUpcomingCommand_ListsLiveAndUpcomingMatches(t *testing.T) {
	bot, session := newInteractionTestBot(t)

	bot.newInteractionHandler(session, makeCommandInteraction("upcoming"))

	text := containerText(t, onlyResponse(t, session))
	if !strings.Contains(text, "## Live Now") || !strings.Contains(text, "**Team A** vs **Team B** (Bo3)\nLIVE") {
		t.Errorf("expected the started, unfinished match listed as live, got:\n%s", text)
	}
}

func TestUpcomingCommand_NothingScheduled(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).SetScheduledMatches(nil)

	bot.newInteractionHandler(session, makeCommandInteraction("upcoming"))

	if text := containerText(t, onlyResponse(t, session)); !strings.Contains(text, "No upcoming matches") {
		t.Errorf("expected the empty-schedule message, got:\n%s", text)
	}
}

func TestTeamsCommand_SortsByVRSRanking(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	ms := bot.APIPtr.Store.(*app.MockStore)
	ms.ValidTeams = []string{"Unranked", "Team B", "Team A"}

	bot.newInteractionHandler(session, makeCommandInteraction("teams"))

	embed := onlyEmbed(t, session)
	if got := embed.Fields[0].Value; got != "`#1`  Team A\n`#2`  Team B\n" {
		t.Errorf("left column = %q", got)
	}
	if got := embed.Fields[1].Value; got != "-  Unranked\n" {
		t.Errorf("right column = %q", got)
	}
	if !strings.HasPrefix(embed.Footer.Text, "3 teams") {
		t.Errorf("footer = %q", embed.Footer.Text)
	}
}

func TestTeamCommand_ShowsRankingAndRoster(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	bot.APIPtr.Store.(*app.MockStore).SetVRSEntries([]store.VRSEntry{
		{TeamName: "Team A", Standing: 3, Points: 1500, Roster: []string{"p1", "p2"}},
	})

	bot.newInteractionHandler(session, makeCommandInteraction("team", strArg("name", "Team A")))

	embed := onlyEmbed(t, session)
	if embed.Description != "**#3** world ranking\n**1500** VRS Points" {
		t.Errorf("description = %q", embed.Description)
	}
	if embed.Fields[0].Value != "• p1\n• p2\n" {
		t.Errorf("roster = %q", embed.Fields[0].Value)
	}
}

func TestTeamCommand_UnknownTeam(t *testing.T) {
	bot, session := newInteractionTestBot(t)

	bot.newInteractionHandler(session, makeCommandInteraction("team", strArg("name", "Nobody")))

	if got := onlyResponse(t, session).Content; got != "No VRS data found for **Nobody**." {
		t.Errorf("content = %q", got)
	}
}

func TestLeaderboardCommand_RanksBySuccesses(t *testing.T) {
	bot, session := newInteractionTestBot(t)
	ms := bot.APIPtr.Store.(*app.MockStore)
	format := string(tournament.Swiss)
	ms.GuildConfig.Format = &format
	ms.Leaderboard = []store.LeaderboardEntry{
		{UserID: "u2", Username: "Second", Successes: 1, Failed: 2},
		{UserID: "u1", Username: "First", Successes: 4},
	}

	bot.newInteractionHandler(session, makeCommandInteraction("leaderboard"))

	text := containerText(t, onlyResponse(t, session))
	if !strings.Contains(text, "1. **First** - 4 correct, 0 incorrect\n2. **Second** - 1 correct, 2 incorrect\n") {
		t.Errorf("unexpected leaderboard body:\n%s", text)
	}
}

func TestResultsCommand_GroupsSingleElimByRound(t *testing.T) {
	bot, session := newInteractionTestBot(t)

	bot.newInteractionHandler(session, makeCommandInteraction("results"))

	text := containerText(t, onlyResponse(t, session))
	qf := strings.Index(text, "Quarter-finals")
	gf := strings.Index(text, "Grand Final")
	if qf < 0 || gf < 0 || qf > gf {
		t.Errorf("expected Quarter-finals before Grand Final, got:\n%s", text)
	}
	if !strings.Contains(text, "**Team A** vs Team B") {
		t.Errorf("expected the quarter-final winner bolded, got:\n%s", text)
	}
}
