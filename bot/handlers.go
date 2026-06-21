package bot

import (
	"context"
	"errors"
	"fmt"
	"pickems-bot/metrics"
	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/tournament"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
)

func (b *Bot) checkInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	var user models.User
	var report tournament.ScoreReport

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		user = models.User{UserID: i.Member.User.ID, Username: i.Member.User.Username}
		var err error
		report, err = b.APIPtr.CheckPrediction(context.Background(), i.GuildID, i.ChannelID, user)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(session, i.Interaction, fmt.Sprintf("%s does not have any Pick'Ems stored. Use `/set` to set your predictions.", user.Username))
			} else {
				b.logger().Error("failed to check prediction", "user", user.Username, "error", fmt.Errorf("checkInteractionHandler: %w", err))
				respondError(session, i.Interaction, fmt.Sprintf("An error occurred checking %s's Pick'Ems.", user.Username))
			}
			return
		}
	} else {
		userID := options[0].Value.(string)
		target := i.ApplicationCommandData().Resolved.Users[userID]
		displayName := target.GlobalName
		if displayName == "" {
			displayName = target.Username
		}
		user = models.User{UserID: target.ID, Username: displayName}
		var err error
		report, err = b.APIPtr.CheckPrediction(context.Background(), i.GuildID, i.ChannelID, user)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(session, i.Interaction, fmt.Sprintf("No Pick'Ems found for **%s**.", user.Username))
			} else {
				b.logger().Error("failed to check prediction", "user", user.Username, "error", fmt.Errorf("checkInteractionHandler: %w", err))
				respondError(session, i.Interaction, fmt.Sprintf("An error occurred checking %s's Pick'Ems.", user.Username))
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

	info, err := b.APIPtr.GetTournamentInfo(context.Background(), i.GuildID, i.ChannelID)
	if err != nil {
		b.logger().Error("failed to get tournament info", "error", fmt.Errorf("checkInteractionHandler: %w", err))
		respondError(session, i.Interaction, "An unexpected error occurred.")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s's Pick'Ems", user.Username),
		Description: fmt.Sprintf("**%d/%d Correct** (%d Pending)", score.Successes, info.NumTeams, score.Pending),
		Color:       green,
		Fields:      fields,
	}

	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}); err != nil {
		b.logger().Error("failed to respond to check interaction", "user", user.Username, "error", fmt.Errorf("checkInteractionHandler: %w", err))
	}
}

func (b *Bot) setInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	info, err := b.APIPtr.GetTournamentInfo(context.Background(), i.GuildID, i.ChannelID)
	if err != nil {
		b.logger().Error("failed to get tournament info", "error", fmt.Errorf("setInteractionHandler: %w", err))
		respondError(session, i.Interaction, "An error occurred loading the tournament.")
		return
	}

	teams, err := b.APIPtr.GetTeams(context.Background(), i.GuildID, i.ChannelID)
	if err != nil {
		b.logger().Error("failed to get teams", "error", fmt.Errorf("setInteractionHandler: %w", err))
		respondError(session, i.Interaction, "An error occurred loading the team list.")
		return
	}

	var teamOptions []discordgo.SelectMenuOption
	for _, t := range teams {
		teamOptions = append(teamOptions, discordgo.SelectMenuOption{
			Label: t.Name,
			Value: t.Name,
		})
	}

	minVal := 0
	var rows []discordgo.MessageComponent
	switch tournament.Kind(info.Format) {
	case tournament.Swiss:
		rows = []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "set_select_win", Placeholder: "3-0 picks (pick 2)", MinValues: &minVal, MaxValues: 2, Options: teamOptions},
			}},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "set_select_advance", Placeholder: "Top 8 advance picks (pick 6)", MinValues: &minVal, MaxValues: 6, Options: teamOptions},
			}},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "set_select_lose", Placeholder: "0-3 picks (pick 2)", MinValues: &minVal, MaxValues: 2, Options: teamOptions},
			}},
		}
	case tournament.SingleElim:
		rows = []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "set_select_third", Placeholder: "3rd/4th place (pick 2)", MinValues: &minVal, MaxValues: 2, Options: teamOptions},
			}},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "set_select_runnerup", Placeholder: "Runner-up (pick 1)", MinValues: &minVal, MaxValues: 1, Options: teamOptions},
			}},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "set_select_winner", Placeholder: "Winner (pick 1)", MinValues: &minVal, MaxValues: 1, Options: teamOptions},
			}},
		}
	default:
		respondError(session, i.Interaction, "Unsupported tournament format.")
		return
	}

	rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "set_submit", Label: "Submit Pick'Ems", Style: discordgo.PrimaryButton},
	}})

	stateKey := i.GuildID + ":" + i.Member.User.ID
	b.setPredictionMu.Lock()
	b.setPredictionState[stateKey] = setPredictionSession{format: tournament.Kind(info.Format), selections: make(map[string][]string)}
	b.setPredictionMu.Unlock()

	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsEphemeral,
			Content:    fmt.Sprintf("Set your Pick'Ems for **%s** — %s", info.TournamentName, info.Round),
			Components: rows,
		},
	}); err != nil {
		b.logger().Error("failed to respond to set interaction", "error", fmt.Errorf("setInteractionHandler: %w", err))
	}
}

func (b *Bot) teamInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	teamName := i.ApplicationCommandData().Options[0].StringValue()

	entry, err := b.APIPtr.GetTeam(context.Background(), teamName)
	if err != nil {
		b.logger().Error("failed to get team", "team", teamName, "error", fmt.Errorf("teamInteractionHandler: %w", err))
		respondError(session, i.Interaction, fmt.Sprintf("No VRS data found for **%s**.", teamName))
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

	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}); err != nil {
		b.logger().Error("failed to respond to team interaction", "error", fmt.Errorf("teamInteractionHandler: %w", err))
	}
}

func (b *Bot) teamsInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	teams, err := b.APIPtr.GetTeams(context.Background(), i.GuildID, i.ChannelID)
	if err != nil {
		b.logger().Error("failed to get teams", "error", fmt.Errorf("teamsInteractionHandler: %w", err))
		session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "An error occurred getting the teams list.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	sort.Slice(teams, func(a, b int) bool {
		ra, rb := teams[a].VRSRanking, teams[b].VRSRanking
		if ra == 0 {
			return false
		}
		if rb == 0 {
			return true
		}
		return ra < rb
	})

	formatEntry := func(name string, ranking int) string {
		if ranking == 0 {
			return fmt.Sprintf("—  %s\n", name)
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
			Text: fmt.Sprintf("%d teams • VRS world ranking shown", len(teams)),
		},
	}

	session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func (b *Bot) leaderboardInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	leaderboard, err := b.APIPtr.GetLeaderboard(context.Background(), i.GuildID, i.ChannelID)
	if err != nil {
		b.logger().Error("failed to get leaderboard", "error", fmt.Errorf("leaderboardHandler: %w", err))
		respondError(session, i.Interaction, "An error occurred getting the leaderboard.")
		return
	}
	if leaderboard == nil {
		respondError(session, i.Interaction, "There are currently no rankings. Try again later.")
		return
	}

	var sb strings.Builder
	for _, user := range leaderboard {
		fmt.Fprintf(&sb, "%d. **%s** — %d correct, %d incorrect\n",
			user.Rank,
			user.Username,
			user.Successes,
			user.Failures,
		)
	}

	accentColor := green
	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsIsComponentsV2,
			Components: []discordgo.MessageComponent{
				discordgo.Container{
					AccentColor: &accentColor,
					Components: []discordgo.MessageComponent{
						discordgo.TextDisplay{Content: "## Leaderboard"},
						discordgo.TextDisplay{Content: sb.String()},
						discordgo.Separator{},
						discordgo.TextDisplay{Content: "-# Scoring: 3pts correct · 1pt pending · 0pts incorrect • No tiebreakers"},
					},
				},
			},
		},
	}); err != nil {
		b.logger().Error("failed to respond to leaderboard interaction", "error", fmt.Errorf("leaderboardInteractionHandler: %w", err))
	}
}

func (b *Bot) upcomingInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	matches, err := b.APIPtr.GetUpcomingMatches(context.Background(), i.GuildID, i.ChannelID)
	if err != nil {
		b.logger().Error("failed to get upcoming matches", "error", fmt.Errorf("upcomingInteractionHandler: %w", err))
		respondError(session, i.Interaction, "An error occurred getting upcoming matches.")
		return
	}

	var liveTexts, upcomingTexts []string
	for _, match := range matches {
		if match.Team1 == "TBD" || match.Team2 == "TBD" || match.BestOf == "" {
			continue
		}
		line := fmt.Sprintf("**%s** vs **%s** (Bo%s)", match.Team1, match.Team2, match.BestOf)
		if match.Live {
			line += "\nLIVE"
		} else {
			line += fmt.Sprintf("\n<t:%d:F> — <t:%d:R>", match.EpochTime, match.EpochTime)
		}
		if match.StreamURL != "" {
			line += fmt.Sprintf("\n\U0001f4fa [Watch live](%s)", match.StreamURL)
		}
		if match.Live {
			liveTexts = append(liveTexts, line)
		} else {
			upcomingTexts = append(upcomingTexts, line)
		}
	}

	var inner []discordgo.MessageComponent

	if len(liveTexts) == 0 && len(upcomingTexts) == 0 {
		inner = append(inner, discordgo.TextDisplay{Content: "No upcoming matches at this time."})
	} else {
		if len(liveTexts) > 0 {
			inner = append(inner, discordgo.TextDisplay{Content: "## Live Now"})
			for _, t := range liveTexts {
				inner = append(inner, discordgo.TextDisplay{Content: t})
			}
		}
		if len(upcomingTexts) > 0 {
			if len(liveTexts) > 0 {
				divider := true
				spacing := discordgo.SeparatorSpacingSizeSmall
				inner = append(inner, discordgo.Separator{Divider: &divider, Spacing: &spacing})
			}
			inner = append(inner, discordgo.TextDisplay{Content: "## Upcoming"})
			for _, t := range upcomingTexts {
				inner = append(inner, discordgo.TextDisplay{Content: t})
			}
		}
	}

	accentColor := green
	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &accentColor,
			Components:  inner,
		},
	}

	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	}); err != nil {
		b.logger().Error("failed to respond to upcoming matches interaction", "error", fmt.Errorf("upcomingInteractionHandler: %w", err))
	}
}

func (b *Bot) resultsInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	nodes, kind, err := b.APIPtr.GetResults(context.Background(), i.GuildID, i.ChannelID)
	if err != nil {
		b.logger().Error("failed to get results", "error", fmt.Errorf("resultsInteractionHandler: %w", err))
		session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "An error occurred fetching the results.",
			},
		})
		return
	}

	var resp *discordgo.InteractionResponse
	switch kind {
	case tournament.Swiss:
		resp = &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{buildSwissResultEmbed(nodes)},
			},
		}
	case tournament.SingleElim:
		resp = &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:      discordgo.MessageFlagsIsComponentsV2,
				Components: buildSingleElimResultComponents(nodes),
			},
		}
	default:
		session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Unsupported tournament format.",
			},
		})
		return
	}

	if err := session.InteractionRespond(i.Interaction, resp); err != nil {
		b.logger().Error("failed to respond to results interaction", "error", fmt.Errorf("resultsInteractionHandler: %w", err))
	}
}

// buildResultMatchSection returns a Section component for a single match node.
// The text shows the winner bolded (if known); the button accessory shows the score.
func buildResultMatchSection(n sources.MatchNode, buttonID int) discordgo.Section {
	var text string
	switch {
	case n.Winner == n.Team1:
		text = fmt.Sprintf("**%s** vs %s", n.Team1, n.Team2)
	case n.Winner == n.Team2:
		text = fmt.Sprintf("%s vs **%s**", n.Team1, n.Team2)
	default:
		text = fmt.Sprintf("%s vs %s", n.Team1, n.Team2)
	}

	label := "—"
	style := discordgo.SecondaryButton
	switch n.Status {
	case "completed":
		score := n.Score
		if parts := strings.SplitN(score, "-", 2); len(parts) == 2 {
			score = parts[0] + " — " + parts[1]
		}
		label = score
		style = discordgo.SuccessButton
	case "in_progress":
		label = "Live"
		style = discordgo.PrimaryButton
	}

	return discordgo.Section{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: text},
		},
		Accessory: discordgo.Button{
			Label:    label,
			Style:    style,
			Disabled: true,
			CustomID: fmt.Sprintf("result_%d", buttonID),
		},
	}
}

// buildRoundContainer wraps a labelled group of match nodes into a Container.
func buildRoundContainer(label string, nodes []sources.MatchNode, idxOffset int) discordgo.Container {
	grey := 0x2b2d31
	comps := []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: "## " + label},
		discordgo.Separator{},
	}
	for j, n := range nodes {
		comps = append(comps, buildResultMatchSection(n, idxOffset+j))
	}
	return discordgo.Container{AccentColor: &grey, Components: comps}
}

// canonicalElimRound maps PandaScore and Liquipedia section labels to a
// consistent display name used for grouping and ordering.
func canonicalElimRound(section string) string {
	lower := strings.ToLower(section)
	switch {
	case strings.HasPrefix(lower, "grand final"):
		return "Grand Final"
	case strings.HasPrefix(lower, "semifinal"), strings.HasPrefix(lower, "semi-final"), strings.HasPrefix(lower, "semi final"):
		return "Semi-finals"
	case strings.HasPrefix(lower, "quarterfinal"), strings.HasPrefix(lower, "quarter-final"), strings.HasPrefix(lower, "quarter final"):
		return "Quarter-finals"
	case strings.HasPrefix(lower, "round of 16"):
		return "Round of 16"
	case strings.HasPrefix(lower, "round of 32"):
		return "Round of 32"
	default:
		return section
	}
}

var singleElimRoundOrder = []string{"Round of 32", "Round of 16", "Quarter-finals", "Semi-finals", "Grand Final"}

func buildSingleElimResultComponents(nodes []sources.MatchNode) []discordgo.MessageComponent {
	byRound := make(map[string][]sources.MatchNode)
	for _, n := range nodes {
		label := canonicalElimRound(n.Section)
		byRound[label] = append(byRound[label], n)
	}

	var containers []discordgo.MessageComponent
	idx := 0
	for _, round := range singleElimRoundOrder {
		matches, ok := byRound[round]
		if !ok {
			continue
		}
		containers = append(containers, buildRoundContainer(round, matches, idx))
		idx += len(matches)
	}
	return containers
}

func buildSwissResultEmbed(nodes []sources.MatchNode) *discordgo.MessageEmbed {
	byRound := make(map[string][]sources.MatchNode)
	var roundOrder []string
	seen := make(map[string]bool)
	for _, n := range nodes {
		if !seen[n.Section] {
			seen[n.Section] = true
			roundOrder = append(roundOrder, n.Section)
		}
		byRound[n.Section] = append(byRound[n.Section], n)
	}

	sort.Slice(roundOrder, func(a, b int) bool {
		var na, nb int
		fmt.Sscanf(roundOrder[a], "Round %d", &na)
		fmt.Sscanf(roundOrder[b], "Round %d", &nb)
		return na < nb
	})

	var fields []*discordgo.MessageEmbedField
	for _, round := range roundOrder {
		var lines []string
		for _, n := range byRound[round] {
			t1, t2 := n.Team1, n.Team2
			if n.Winner == n.Team1 {
				t1 = "**" + t1 + "**"
			} else if n.Winner == n.Team2 {
				t2 = "**" + t2 + "**"
			}
			score := n.Score
			if parts := strings.SplitN(score, "-", 2); len(parts) == 2 {
				score = parts[0] + " — " + parts[1]
			}
			if score != "" {
				lines = append(lines, fmt.Sprintf("%s vs %s (%s)", t1, t2, score))
			} else {
				lines = append(lines, fmt.Sprintf("%s vs %s", t1, t2))
			}
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  round,
			Value: strings.Join(lines, "\n"),
		})
	}

	return &discordgo.MessageEmbed{
		Title:  "Results",
		Color:  0x5865F2,
		Fields: fields,
	}
}

func (b *Bot) newAutocompleteInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	switch i.ApplicationCommandData().Name {
	case "team":
		b.teamNameAutocomplete(session, i)
	}
}

func (b *Bot) teamNameAutocomplete(session DiscordSession, i *discordgo.InteractionCreate) {
	var typed string
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Focused {
			typed = strings.ToLower(opt.StringValue())
			break
		}
	}

	teams, err := b.APIPtr.GetTeams(context.Background(), i.GuildID, i.ChannelID)
	if err != nil {
		b.logger().Error("failed to get teams for autocomplete", "error", fmt.Errorf("teamNameAutocomplete: %w", err))
		session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	for _, t := range teams {
		if typed == "" || strings.Contains(strings.ToLower(t.Name), typed) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  t.Name,
				Value: t.Name,
			})
			if len(choices) == 25 {
				break
			}
		}
	}

	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	}); err != nil {
		b.logger().Error("failed to respond to team autocomplete", "error", fmt.Errorf("teamNameAutocomplete: %w", err))
	}
}

func (b *Bot) newComponentHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	switch {
	case strings.HasPrefix(customID, "set_select_"):
		b.setSelectHandler(session, i)
	case customID == "set_submit":
		b.setSubmitHandler(session, i)
	}
}

func (b *Bot) setSelectHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	stateKey := i.GuildID + ":" + i.Member.User.ID
	customID := i.MessageComponentData().CustomID
	values := i.MessageComponentData().Values

	b.setPredictionMu.Lock()
	state, ok := b.setPredictionState[stateKey]
	if ok {
		state.selections[customID] = values
		b.setPredictionState[stateKey] = state
	}
	b.setPredictionMu.Unlock()

	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		b.logger().Error("failed to ack select interaction", "error", fmt.Errorf("setSelectHandler: %w", err))
	}
}

func (b *Bot) setSubmitHandler(session DiscordSession, i *discordgo.InteractionCreate) {
	stateKey := i.GuildID + ":" + i.Member.User.ID

	b.setPredictionMu.Lock()
	state, ok := b.setPredictionState[stateKey]
	b.setPredictionMu.Unlock()

	ephemeralError := func(msg string) {
		session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: msg,
			},
		})
	}

	if !ok {
		ephemeralError("Session expired — please run `/set` again.")
		return
	}

	var userPreds []string
	switch state.format {
	case tournament.Swiss:
		userPreds = append(userPreds, state.selections["set_select_win"]...)
		userPreds = append(userPreds, state.selections["set_select_advance"]...)
		userPreds = append(userPreds, state.selections["set_select_lose"]...)
	case tournament.SingleElim:
		userPreds = append(userPreds, state.selections["set_select_third"]...)
		userPreds = append(userPreds, state.selections["set_select_runnerup"]...)
		userPreds = append(userPreds, state.selections["set_select_winner"]...)
	}

	seen := make(map[string]bool)
	for _, p := range userPreds {
		if seen[p] {
			ephemeralError(fmt.Sprintf("**%s** appears in multiple buckets — each team can only be picked once.", p))
			return
		}
		seen[p] = true
	}

	// All local validation passed — defer before the DB call
	if err := session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		b.logger().Error("failed to defer set submit", "error", fmt.Errorf("setSubmitHandler: %w", err))
		return
	}

	errContent := func(msg string) {
		session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
	}

	user := models.User{UserID: i.Member.User.ID, Username: i.Member.User.Username}
	prediction, err := b.APIPtr.SetUserPrediction(context.Background(), i.GuildID, i.ChannelID, user, userPreds)
	if err != nil {
		b.logger().Error("failed to set user prediction", "user", user.Username, "error", fmt.Errorf("setSubmitHandler: %w", err))
		msg := fmt.Sprintf("Failed to save Pick'Ems: %s", err.Error())
		errContent(msg)
		return
	}

	b.setPredictionMu.Lock()
	delete(b.setPredictionState, stateKey)
	b.setPredictionMu.Unlock()

	fields, err := predictionFields(prediction)
	if err != nil {
		b.logger().Error("failed to build prediction fields", "user", user.Username, "error", fmt.Errorf("setSubmitHandler: %w", err))
		errContent("An error occurred displaying your Pick'Ems.")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Pick'Ems Updated",
		Description: fmt.Sprintf("%s's Pick'Ems have been saved.", user.Username),
		Color:       green,
		Fields:      fields,
	}
	embeds := []*discordgo.MessageEmbed{embed}
	if _, err := session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &embeds,
	}); err != nil {
		b.logger().Error("failed to edit set submit response", "error", fmt.Errorf("setSubmitHandler: %w", err))
	}
}

func (b *Bot) newInteractionHandler(session DiscordSession, i *discordgo.InteractionCreate) {

	switch i.Type {
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.newAutocompleteInteractionHandler(session, i)
		return
	case discordgo.InteractionMessageComponent:
		b.newComponentHandler(session, i)
		return
	}

	// Route to appropriate handler
	switch i.ApplicationCommandData().Name {
	case "check":
		metrics.DiscordCommandsTotal.WithLabelValues("check").Inc()
		b.checkInteractionHandler(session, i)
	case "set":
		metrics.DiscordCommandsTotal.WithLabelValues("set").Inc()
		b.setInteractionHandler(session, i)
	case "team":
		metrics.DiscordCommandsTotal.WithLabelValues("team").Inc()
		b.teamInteractionHandler(session, i)
	case "teams":
		metrics.DiscordCommandsTotal.WithLabelValues("teams").Inc()
		b.teamsInteractionHandler(session, i)
	case "leaderboard":
		metrics.DiscordCommandsTotal.WithLabelValues("leaderboard").Inc()
		b.leaderboardInteractionHandler(session, i)
	case "upcoming":
		metrics.DiscordCommandsTotal.WithLabelValues("upcoming").Inc()
		b.upcomingInteractionHandler(session, i)
	case "results":
		metrics.DiscordCommandsTotal.WithLabelValues("results").Inc()
		b.resultsInteractionHandler(session, i)
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
