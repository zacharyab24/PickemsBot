/* handlers.go
 * Contains testable handler methods that accept DiscordSession interface
 * Authors: Zachary Bower
 * AI-Generated: Extracted runtime functionality from bot.go
 */

package bot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"pickems-bot/metrics"
	"pickems-bot/models"
	"pickems-bot/tournament"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/go-andiamo/splitter"
	"github.com/jackc/pgx/v5"
)

// helpMessageHandler handles the $help command with a DiscordSession interface
func (b *Bot) helpMessageHandler(session DiscordSession, message *discordgo.MessageCreate) {
	embed := &discordgo.MessageEmbed{
		Title: "PickEms Bot v3.3",
		Description: "Manage your tournament predictions and check standings. All commands use the `$` prefix.\n\n" +
			"*Match Data sourced from the [Liquipedia Counter-Strike API](https://liquipedia.net) and [PandaScore](https://pandascore.co)*\n" +
			"*VRS Data sourced from the [counter-strike_regional_standings](https://github.com/ValveSoftware/counter-strike_regional_standings) GitHub repo*",
		Color: burple,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "`$details`",
				Value:  "Check active tournament info (name, current round, format, and team requirements).",
				Inline: false,
			},
			{
				Name: "`$set <team1> ... <teamN>`",
				Value: cleanIndent(`Lock in your tournament predictions.
			- **Swiss:** 10 teams needed (1-2: 3-0 teams | 3-8: top-8 | 9-10: 0-3 teams).
			- **Single Elim:** 4 teams needed (1-2: 3rd/4th place | 3: runner-up | 4: winner).
			- **Tip:** Wrap multi-word names in quotes (e.g., \"The MongolZ\").`),
				Inline: false,
			},
			{
				Name:   "`$check`",
				Value:  "View your currently saved Pick'Ems",
				Inline: false,
			},
			{
				Name:   "`$teams`",
				Value:  "List all teams alive in the current stage. Use these exact names for the `$set` command if fuzzy matching doesn't work.",
				Inline: false,
			},
			{
				Name:   "`$team <name>`",
				Value:  "Look up a team's current VRS world ranking and roster.",
				Inline: false,
			},
			{
				Name:   "`$leaderboard`",
				Value:  "See who has the most correct picks this stage. Sorted strictly by total wins (no tiebreakers).",
				Inline: false,
			},
			{
				Name:   "`$upcoming`",
				Value:  "Show matches upcoming matches for this round of the tournament.",
				Inline: false,
			},
			{
				Name: "`$results`",
				Value: cleanIndent(`Generate a visual bracket image for Swiss or Single Elimination stages.
			*Note: Third-place matches are hidden in Single Elim brackets.*`),

				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Fuzzy matching is active, but keep names as close as possible!",
		},
	}

	if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
		b.logger().Error("failed to send help embed", "error", fmt.Errorf("helpMessageHandler: %w", err))
	}
}

// detailsHandler handles the $details command with a DiscordSession interface
func (b *Bot) detailsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	info, err := b.APIPtr.GetTournamentInfo(context.Background(), message.GuildID, message.ChannelID)
	if err != nil {
		b.logger().Error("failed to get tournament info", "error", fmt.Errorf("detailsHandler: %w", err))
		sendError(session, message.ChannelID, "An unexpected error occurred.")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "Match Details",
		Color: green,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Tournament Name",
				Value:  info.TournamentName,
				Inline: false,
			},
			{
				Name:   "Round",
				Value:  info.Round,
				Inline: false,
			},
			{
				Name:   "Format",
				Value:  info.Format,
				Inline: false,
			},
			{
				Name:   "Number of Required Teams",
				Value:  strconv.Itoa(info.NumTeams),
				Inline: false,
			},
		},
	}

	if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
		b.logger().Error("failed to send details embed", "error", fmt.Errorf("detailsHandler: %w", err))
	}
}

// setPredictionsHandler handles the $set command with a DiscordSession interface
func (b *Bot) setPredictionsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	user := models.User{UserID: message.Author.ID, Username: message.Author.Username}

	// Get User Predictions from message
	spaceSplitter, _ := splitter.NewSplitter(' ', splitter.DoubleQuotes, splitter.LeftRightDoubleDoubleQuotes)
	msg, _ := spaceSplitter.Split(message.Content)
	userPreds := msg[1:]

	prediction, err := b.APIPtr.SetUserPrediction(context.Background(), message.GuildID, message.ChannelID, user, userPreds)
	if err != nil {
		b.logger().Error("failed to set user prediction", "user", user.Username, "error", fmt.Errorf("setPredictionsHandler: %w", err))
		sendError(session, message.ChannelID, err.Error())
		return
	}

	fields, err := predictionFields(prediction)
	if err != nil {
		b.logger().Error("failed to build prediction fields", "user", user.Username, "error", fmt.Errorf("setPredictionsHandler: %w", err))
		sendError(session, message.ChannelID, "An error occurred displaying your Pick'Ems.")
		return
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Pick'Ems Updated",
		Description: fmt.Sprintf("%s's Pick'Ems have been saved.", user.Username),
		Color:       green,
		Fields:      fields,
	}
	if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
		b.logger().Error("failed to send set-predictions embed", "error", fmt.Errorf("setPredictionsHandler: %w", err))
	}
}

func predictionFields(p models.Prediction) ([]*discordgo.MessageEmbedField, error) {
	f, err := tournament.Get(tournament.Kind(p.Format))
	if err != nil {
		return nil, fmt.Errorf("unknown prediction format %q", p.Format)
	}
	pFields, err := f.PredictionFields(p)
	if err != nil {
		return nil, err
	}
	out := make([]*discordgo.MessageEmbedField, len(pFields))
	for i, pf := range pFields {
		out[i] = &discordgo.MessageEmbedField{Name: pf.Name, Value: pf.Value}
	}
	return out, nil
}

// checkPredictionsHandler handles the $check command with a DiscordSession interface
func (b *Bot) checkPredictionsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	var user models.User
	var report tournament.ScoreReport

	target := strings.TrimSpace(strings.TrimPrefix(message.Content, "$check"))
	if target == "" {
		user = models.User{UserID: message.Author.ID, Username: message.Author.Username}
		var err error
		report, err = b.APIPtr.CheckPrediction(context.Background(), message.GuildID, message.ChannelID, user)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sendError(session, message.ChannelID, fmt.Sprintf("%s does not have any Pick'Ems stored. Use `$set` to set your predictions.", user.Username))
			} else {
				b.logger().Error("failed to check prediction", "user", user.Username, "error", fmt.Errorf("checkPredictionsHandler: %w", err))
				sendError(session, message.ChannelID, fmt.Sprintf("An error occurred checking %s's Pick'Ems.", user.Username))
			}
			return
		}
	} else {
		var err error
		user, report, err = b.APIPtr.CheckPredictionByUsername(context.Background(), message.GuildID, message.ChannelID, target)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sendError(session, message.ChannelID, fmt.Sprintf("No Pick'Ems found for **%s**.", target))
			} else {
				b.logger().Error("failed to check prediction by username", "target", target, "error", fmt.Errorf("checkPredictionsHandler: %w", err))
				sendError(session, message.ChannelID, fmt.Sprintf("An error occurred checking %s's Pick'Ems.", target))
			}
			return
		}
	}

	score := report.GetScore()
	var fields []*discordgo.MessageEmbedField

	switch r := report.(type) {
	case tournament.SwissReport:
		fields = append(fields, swissBucketField("**3-0**", r.WinPicks))
		fields = append(fields, swissBucketField("**Advance**", r.AdvancePicks))
		fields = append(fields, swissBucketField("**0-3**", r.LosePicks))
	case tournament.SingleElimReport:
		fields = append(fields, singleElimField(r.Predictions))
	}

	info, err := b.APIPtr.GetTournamentInfo(context.Background(), message.GuildID, message.ChannelID)
	if err != nil {
		b.logger().Error("failed to get tournament info", "error", fmt.Errorf("checkPredictionsHandler: %w", err))
		sendError(session, message.ChannelID, "An unexpected error occurred.")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s's Pick'Ems", user.Username),
		Description: fmt.Sprintf("**%d/%d Correct** (%d Pending)", score.Successes, info.NumTeams, score.Pending),
		Color:       green,
		Fields:      fields,
	}

	if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
		b.logger().Error("failed to send check-predictions embed", "user", user.Username, "error", fmt.Errorf("checkPredictionsHandler: %w", err))
	}
}

// leaderboardHandler handles the $leaderboard command with a DiscordSession interface
func (b *Bot) leaderboardHandler(session DiscordSession, message *discordgo.MessageCreate) {
	leaderboard, err := b.APIPtr.GetLeaderboard(context.Background(), message.GuildID, message.ChannelID)
	if err != nil {
		b.logger().Error("failed to get leaderboard", "error", fmt.Errorf("leaderboardHandler: %w", err))
		sendError(session, message.ChannelID, "An error occurred getting the leaderboard.")
		return
	}
	if leaderboard == nil {
		sendError(session, message.ChannelID, "There are currently no rankings. Try again later.")
		return
	}

	var sb strings.Builder
	for _, user := range leaderboard {
		fmt.Fprintf(&sb, "%d. %s - %d Successes, %d Failures\n",
			user.Rank,
			user.Username,
			user.Successes,
			user.Failures,
		)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Leaderboard",
		Description: sb.String(),
		Color:       green,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Calculated using (Successes * 3) + (Pending * 1) + (Failed * 0) • No tiebreakers applied",
		},
	}

	if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
		b.logger().Error("failed to send leaderboard embed", "error", fmt.Errorf("leaderboardHandler: %w", err))
	}
}

// teamsHandler handles the $teams command with a DiscordSession interface
func (b *Bot) teamsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	teams, err := b.APIPtr.GetTeams(context.Background(), message.GuildID, message.ChannelID)
	if err != nil {
		b.logger().Error("failed to get teams", "error", fmt.Errorf("teamsHandler: %w", err))
		sendError(session, message.ChannelID, "An error occurred getting the teams list.")
		return
	}

	// Sort by VRS ranking ascending; unranked teams (0) go to the end
	sort.Slice(teams, func(i, j int) bool {
		ri, rj := teams[i].VRSRanking, teams[j].VRSRanking
		if ri == 0 {
			return false
		}
		if rj == 0 {
			return true
		}
		return ri < rj
	})

	formatEntry := func(name string, ranking int) string {
		if ranking == 0 {
			return fmt.Sprintf("\u2014  %s\n", name)
		}
		return fmt.Sprintf("`#%d`  %s\n", ranking, name)
	}

	mid := (len(teams) + 1) / 2
	var left, right strings.Builder
	for _, t := range teams[:mid] {
		left.WriteString(formatEntry(t.Name, t.VRSRanking))
	}
	for _, t := range teams[mid:] {
		right.WriteString(formatEntry(t.Name, t.VRSRanking))
	}

	embed := &discordgo.MessageEmbed{
		Title: "Teams in this Stage",
		Color: green,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "\u200b", Value: left.String(), Inline: true},
			{Name: "\u200b", Value: right.String(), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%d teams \u2022 VRS world ranking shown \u2022 Fuzzy matching is active", len(teams)),
		},
	}

	if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
		b.logger().Error("failed to send teams embed", "error", fmt.Errorf("teamsHandler: %w", err))
	}
}

// teamHandler handles the $team <name> command with a DiscordSession interface
func (b *Bot) teamHandler(session DiscordSession, message *discordgo.MessageCreate) {
	spaceSplitter, _ := splitter.NewSplitter(' ', splitter.DoubleQuotes, splitter.LeftRightDoubleDoubleQuotes)
	msg, _ := spaceSplitter.Split(message.Content)
	if len(msg) < 2 {
		sendError(session, message.ChannelID, "Usage: `$team <team name>`")
		return
	}
	teamName := strings.Join(msg[1:], " ")

	entry, err := b.APIPtr.GetTeam(context.Background(), teamName)
	if err != nil {
		b.logger().Error("failed to get team", "team", teamName, "error", fmt.Errorf("teamHandler: %w", err))
		sendError(session, message.ChannelID, fmt.Sprintf("No VRS data found for **%s**.", teamName))
		return
	}

	var rosterLines strings.Builder
	for _, player := range entry.Roster {
		fmt.Fprintf(&rosterLines, "• %s\n", player)
	}

	footerText := "VRS Rankings"
	if !entry.StandingsDate.IsZero() {
		footerText = "Rankings as of " + entry.StandingsDate.Format("02 Jan 2006")
	}

	embed := &discordgo.MessageEmbed{
		Title:       entry.TeamName,
		Description: fmt.Sprintf("**#%d** world ranking\n**%d** VRS Points", entry.Standing, entry.Points),
		Color:       green,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Roster", Value: rosterLines.String(), Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: footerText},
	}

	if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
		b.logger().Error("failed to send team embed", "error", fmt.Errorf("teamHandler: %w", err))
	}
}

// upcomingMatchesHandler handles the $upcoming command with a DiscordSession interface
func (b *Bot) upcomingMatchesHandler(session DiscordSession, message *discordgo.MessageCreate) {
	matches, err := b.APIPtr.GetUpcomingMatches(context.Background(), message.GuildID, message.ChannelID)
	if err != nil {
		b.logger().Error("failed to get upcoming matches", "error", fmt.Errorf("upcomingMatchesHandler: %w", err))
		sendError(session, message.ChannelID, "An error occurred getting upcoming matches.")
		return
	}

	if len(matches) == 0 {
		embed := &discordgo.MessageEmbed{
			Title:       "Upcoming Matches",
			Description: "No upcoming matches at this time.",
			Color:       green,
		}
		if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
			b.logger().Error("failed to send upcoming-matches embed", "error", fmt.Errorf("upcomingMatchesHandler: %w", err))
		}
		return
	}

	var liveFields, upcomingFields []*discordgo.MessageEmbedField
	for _, match := range matches {
		if match.Team1 == "TBD" || match.Team2 == "TBD" {
			continue
		}
		name := fmt.Sprintf("**%s** vs **%s** (Bo%s)", match.Team1, match.Team2, match.BestOf)
		var value string
		if match.Live {
			value = "**LIVE**"
		} else {
			value = fmt.Sprintf("<t:%d:F> — <t:%d:R>", match.EpochTime, match.EpochTime)
		}
		if match.StreamURL != "" {
			value += fmt.Sprintf("\n📺 [Watch live](%s)", match.StreamURL)
		}
		field := &discordgo.MessageEmbedField{Name: name, Value: value}
		if match.Live {
			liveFields = append(liveFields, field)
		} else {
			upcomingFields = append(upcomingFields, field)
		}
	}

	var fields []*discordgo.MessageEmbedField
	if len(liveFields) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "🔴  Live Now", Value: "\u200b"})
		fields = append(fields, liveFields...)
	}
	if len(upcomingFields) > 0 {
		if len(liveFields) > 0 {
			fields = append(fields, &discordgo.MessageEmbedField{Name: "Upcoming", Value: "\u200b"})
		}
		fields = append(fields, upcomingFields...)
	}

	if len(fields) == 0 {
		embed := &discordgo.MessageEmbed{
			Title:       "Upcoming Matches",
			Description: "No upcoming matches at this time.",
			Color:       green,
		}
		if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
			b.logger().Error("failed to send upcoming-matches embed", "error", fmt.Errorf("upcomingMatchesHandler: %w", err))
		}
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:  "Upcoming Matches",
		Color:  green,
		Fields: fields,
	}

	if _, err := session.ChannelMessageSendEmbed(message.ChannelID, embed); err != nil {
		b.logger().Error("failed to send upcoming-matches embed", "error", fmt.Errorf("upcomingMatchesHandler: %w", err))
	}
}

// resultsHandler handles the $results command withing a DiscordSession interface
// the results image should be stored in <project-root>/resources/result.png.
// Creating / updating the results image is a slow process and should be handled when we update the match results db via a goroutine
func (b *Bot) resultsHandler(session DiscordSession, message *discordgo.MessageCreate) {
	outputPath := "resources/result.png"

	// Load image from disk
	f, err := os.Open(outputPath)
	if err != nil {
		b.logger().Error("failed to open results image", "path", outputPath, "error", fmt.Errorf("resultsHandler: %w", err))
		sendError(session, message.ChannelID, "An error occurred fetching the match results.")
		return
	}

	session.ChannelFileSend(message.ChannelID, outputPath, f)
}

// newMessageHandler routes messages to appropriate handlers with a DiscordSession interface
// botUserID is the bot's user ID to prevent self-responses
func (b *Bot) newMessageHandler(session DiscordSession, message *discordgo.MessageCreate, botUserID string) {
	// Prevent bot from responding to its own messages
	if message.Author.ID == botUserID {
		return
	}

	// Route to appropriate handler
	switch {
	case startsWith(message.Content, "$help"):
		metrics.DiscordCommandsTotal.WithLabelValues("help").Inc()
		b.helpMessageHandler(session, message)

	case startsWith(message.Content, "$details"):
		metrics.DiscordCommandsTotal.WithLabelValues("details").Inc()
		b.detailsHandler(session, message)

	case startsWith(message.Content, "$set"):
		metrics.DiscordCommandsTotal.WithLabelValues("set").Inc()
		b.setPredictionsHandler(session, message)

	case startsWith(message.Content, "$check"):
		metrics.DiscordCommandsTotal.WithLabelValues("check").Inc()
		b.checkPredictionsHandler(session, message)

	case startsWith(message.Content, "$leaderboard"):
		metrics.DiscordCommandsTotal.WithLabelValues("leaderboard").Inc()
		b.leaderboardHandler(session, message)

	case startsWith(message.Content, "$teams"):
		metrics.DiscordCommandsTotal.WithLabelValues("teams").Inc()
		b.teamsHandler(session, message)

	case startsWith(message.Content, "$team "):
		metrics.DiscordCommandsTotal.WithLabelValues("team").Inc()
		b.teamHandler(session, message)

	case startsWith(message.Content, "$upcoming"):
		metrics.DiscordCommandsTotal.WithLabelValues("upcoming").Inc()
		b.upcomingMatchesHandler(session, message)

	case startsWith(message.Content, "$result"):
		metrics.DiscordCommandsTotal.WithLabelValues("results").Inc()
		b.resultsHandler(session, message)
	}
}
