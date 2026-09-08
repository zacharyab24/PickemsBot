package bot

import (
	"fmt"
	"log/slog"
	format "pickems-bot/tournament"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const green = 0x57F287
const red = 0xED4245

// singleElimField formats a single-elimination predictions list as an embed field,
// ordered Champion → Runner-up → 3rd/4th → … .
func singleElimField(entries []format.ElimPredictionEntry) *discordgo.MessageEmbedField {
	sorted := make([]format.ElimPredictionEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		ri := elimRoundOrder[sorted[i].Round]
		rj := elimRoundOrder[sorted[j].Round]
		if ri != rj {
			return ri < rj
		}
		// Within the same round: winner before loser
		if sorted[i].ToWin != sorted[j].ToWin {
			return sorted[i].ToWin
		}
		return sorted[i].Team < sorted[j].Team // stable alphabetical tiebreak
	})

	var sb strings.Builder
	for _, e := range sorted {
		sb.WriteString(fmt.Sprintf("%s: **%s** %s\n", elimPositionLabel(e), e.Team, e.Status))
	}
	if sb.Len() == 0 {
		sb.WriteString("—")
	}
	return &discordgo.MessageEmbedField{Name: "**Predictions**", Value: sb.String(), Inline: false}
}

// elimRoundOrder defines the sort priority for each single-elim round.
// Lower value = displayed first (Grand Final winner at the top).
var elimRoundOrder = map[string]int{
	"Grand Final":   0,
	"Semi Final":    1,
	"Quarter Final": 2,
	"Best of 16":    3,
	"Best of 32":    4,
}

// elimPositionLabel returns the human-readable position label for an entry,
// prefixed with a medal/trophy emoji for the Discord embed.
func elimPositionLabel(e format.ElimPredictionEntry) string {
	if e.ToWin {
		return "🏆 Champion"
	}
	switch e.Round {
	case "Grand Final":
		return "🥈 Runner-up"
	case "Semi Final":
		return "🥉 3rd / 4th"
	case "Quarter Final":
		return "🎖️ Top 8"
	default:
		return e.Round
	}
}

// swissBucketField formats one Swiss prediction bucket (e.g. "3-0") as an embed field.
func swissBucketField(label string, entries []format.BucketEntry) *discordgo.MessageEmbedField {
	var sb strings.Builder
	for _, e := range entries {
		score := e.Score
		if score == "" {
			score = "N/A"
		}
		sb.WriteString(fmt.Sprintf("**%s**: %s %s\n", e.Team, score, e.Status))
	}
	if sb.Len() == 0 {
		sb.WriteString("—")
	}
	return &discordgo.MessageEmbedField{Name: label, Value: sb.String(), Inline: false}
}

// respondError sends an inline error message as an interaction response.
// Use this in slash command handlers; sendError sends a standalone channel message
// which leaves the interaction in a perpetual "thinking" state.
func respondError(session DiscordSession, i *discordgo.Interaction, msg string) {
	session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: msg},
	})
}

// respondEphemeral sends an ephemeral (only visible to the invoking user) message
// as an interaction response. Used by /config so admin output doesn't clutter the channel.
func respondEphemeral(session DiscordSession, i *discordgo.Interaction, msg string) {
	session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// deferEphemeral acknowledges an interaction with an ephemeral "thinking..."
// state, so a handler that does DB work has more than Discord's 3-second
// initial-response window before it has to reply. The caller finalises with
// session.InteractionResponseEdit once its work completes; if it can't defer
// (rare - usually a dead session), the caller should log and bail out rather
// than attempt a slow synchronous InteractionRespond that Discord will most
// likely reject anyway.
func deferEphemeral(session DiscordSession, i *discordgo.Interaction) error {
	return session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})
}

// sendError sends a red error embed to the given channel.
func sendError(session DiscordSession, channelID string, msg string) {
	embed := &discordgo.MessageEmbed{
		Title:       "Error",
		Description: msg,
		Color:       red,
	}
	if _, err := session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		slog.Error("failed to send error embed", "error", fmt.Errorf("sendError: %w", err))
	}
}
