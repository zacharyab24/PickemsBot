/* handlers_test.go
 * Contains unit tests for bot command handlers using mock Discord session
 * AI-Generated
 */

package bot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pickems-bot/app"
	"pickems-bot/models"
	"pickems-bot/sources"
	"pickems-bot/store"
	"pickems-bot/tournament"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestBot creates a Bot instance with a mock API for testing
func createTestBot(kind tournament.Kind) *Bot {
	mockStore := app.NewMockStore(kind, "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3", EpochTime: 1700000000},
		{Team1: "Team C", Team2: "Team D", BestOf: "3", EpochTime: 1700010000},
	})
	mockStore.SetSwissResults(map[string]string{
		"Team A": "3-0",
		"Team B": "3-1",
		"Team C": "3-2",
		"Team D": "2-3",
		"Team E": "1-3",
		"Team F": "0-3",
	})

	return &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}
}

// createTestBotWithElimination creates a Bot with elimination format
func createTestBotWithElimination() *Bot {
	mockStore := app.NewMockStore("single-elimination", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3", EpochTime: 1700000000},
	})
	mockStore.SetEliminationResults(map[string]models.TeamProgress{
		"Team A": {Round: "semifinal", Status: "advanced"},
		"Team B": {Round: "quarterfinal", Status: "eliminated"},
	})

	return &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}
}

// createMockMessage creates a mock Discord message for testing
func createMockMessage(content, userID, username, channelID string) *discordgo.MessageCreate {
	return &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content:   content,
			ChannelID: channelID,
			Author: &discordgo.User{
				ID:       userID,
				Username: username,
			},
		},
	}
}

// region helpMessage tests

func TestHelpMessage_Success(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$help", "user123", "TestUser", "channel123")

	bot.helpMessageHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
	assert.Contains(t, msg.Content, "PickEms Bot")
	assert.Contains(t, msg.Content, "$set")
	assert.Contains(t, msg.Content, "$check")
	assert.Contains(t, msg.Content, "$leaderboard")
}

// endregion

// region details tests

func TestDetails_SwissFormat(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$details", "user123", "TestUser", "channel123")

	bot.detailsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
	// The message should contain tournament info
	assert.NotEmpty(t, msg.Content)
}

func TestDetails_EliminationFormat(t *testing.T) {
	bot := createTestBotWithElimination()
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$details", "user123", "TestUser", "channel123")

	bot.detailsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
}

// endregion

// region teams tests

func TestTeams_Success(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$teams", "user123", "TestUser", "channel123")

	bot.teamsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
	assert.Contains(t, msg.Content, "Teams in this Stage")
	assert.Contains(t, msg.Content, "Team")
}

func TestTeams_ShowsVRSRankInFooter(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$teams", "user123", "TestUser", "channel123")

	bot.teamsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	embed := mockSession.GetLastEmbed()
	require.NotNil(t, embed)
	assert.Contains(t, embed.Embed.Footer.Text, "VRS world ranking shown")
}

// endregion

// region team tests

func TestTeam_Success(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{
		{
			TeamName:      "FaZe",
			Standing:      5,
			Points:        1750,
			Roster:        []string{"karrigan", "broky", "rain", "frozen", "ropz"},
			StandingsDate: "2026_05_01",
		},
	})
	b := &Bot{BotToken: "test_token", APIPtr: &app.App{Store: mockStore}}
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$team faze", "user123", "TestUser", "channel123")

	b.teamHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, msg.Content, "FaZe")
	assert.Contains(t, msg.Content, "#5")
	assert.Contains(t, msg.Content, "karrigan")
}

func TestTeam_MissingArg(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	b := &Bot{BotToken: "test_token", APIPtr: &app.App{Store: mockStore}}
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$team", "user123", "TestUser", "channel123")

	b.teamHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "usage")
}

func TestTeam_NotFound(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetVRSEntries([]store.VRSEntry{
		{TeamName: "FaZe", Standing: 5},
	})
	b := &Bot{BotToken: "test_token", APIPtr: &app.App{Store: mockStore}}
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$team unknownteamxyz", "user123", "TestUser", "channel123")

	b.teamHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "no vrs data")
}

// endregion

// region upcomingMatches tests

func TestUpcomingMatches_Success(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$upcoming", "user123", "TestUser", "channel123")

	bot.upcomingMatchesHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
	// Note: The bot filters out matches with "TBD" so it may show "No upcoming matches"
	// if the test data doesn't have non-TBD matches
	assert.NotEmpty(t, msg.Content)
}

func TestUpcomingMatches_LiveOnly_HasLiveNowHeader(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3", EpochTime: time.Now().Add(-1 * time.Hour).Unix(), Finished: false},
	})
	bot := &Bot{BotToken: "test_token", APIPtr: &app.App{Store: mockStore}}
	mockSession := NewMockDiscordSession()

	bot.upcomingMatchesHandler(mockSession, createMockMessage("$upcoming", "user123", "TestUser", "channel123"))

	require.Len(t, mockSession.SentEmbeds, 1)
	fields := mockSession.GetLastEmbed().Embed.Fields
	require.Len(t, fields, 2)
	assert.Equal(t, "🔴  Live Now", fields[0].Name)
	assert.Equal(t, "**Team A** vs **Team B** (Bo3)", fields[1].Name)
	assert.Contains(t, fields[1].Value, "**LIVE**")
}

func TestUpcomingMatches_UpcomingOnly_NoSectionHeaders(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3", EpochTime: time.Now().Add(24 * time.Hour).Unix(), Finished: false},
	})
	bot := &Bot{BotToken: "test_token", APIPtr: &app.App{Store: mockStore}}
	mockSession := NewMockDiscordSession()

	bot.upcomingMatchesHandler(mockSession, createMockMessage("$upcoming", "user123", "TestUser", "channel123"))

	require.Len(t, mockSession.SentEmbeds, 1)
	fields := mockSession.GetLastEmbed().Embed.Fields
	require.Len(t, fields, 1)
	assert.Equal(t, "**Team A** vs **Team B** (Bo3)", fields[0].Name)
}

func TestUpcomingMatches_LiveAndUpcoming_BothSectionHeaders(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3", EpochTime: time.Now().Add(-1 * time.Hour).Unix(), Finished: false},
		{Team1: "Team C", Team2: "Team D", BestOf: "3", EpochTime: time.Now().Add(24 * time.Hour).Unix(), Finished: false},
	})
	bot := &Bot{BotToken: "test_token", APIPtr: &app.App{Store: mockStore}}
	mockSession := NewMockDiscordSession()

	bot.upcomingMatchesHandler(mockSession, createMockMessage("$upcoming", "user123", "TestUser", "channel123"))

	require.Len(t, mockSession.SentEmbeds, 1)
	fields := mockSession.GetLastEmbed().Embed.Fields
	// Expected order: Live Now header, live match, Upcoming header, upcoming match
	require.Len(t, fields, 4)
	assert.Equal(t, "🔴  Live Now", fields[0].Name)
	assert.Equal(t, "**Team A** vs **Team B** (Bo3)", fields[1].Name)
	assert.Contains(t, fields[1].Value, "**LIVE**")
	assert.Equal(t, "Upcoming", fields[2].Name)
	assert.Equal(t, "**Team C** vs **Team D** (Bo3)", fields[3].Name)
}

func TestUpcomingMatches_AllTBD_ShowsNoMatchesEmbed(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	// EpochTime in the future so the match isn't filtered by GetUpcomingMatches,
	// but both teams are TBD so the render loop skips it.
	futureTime := time.Now().Add(24 * time.Hour).Unix()
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "TBD", Team2: "TBD", BestOf: "3", EpochTime: futureTime},
	})

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$upcoming", "user123", "TestUser", "channel123")

	bot.upcomingMatchesHandler(mockSession, message)

	require.Len(t, mockSession.SentEmbeds, 1)
	embed := mockSession.GetLastEmbed()
	assert.Equal(t, "Upcoming Matches", embed.Embed.Title)
	assert.Equal(t, "No upcoming matches at this time.", embed.Embed.Description)
}

// endregion

// region leaderboard tests

func TestLeaderboard_Success(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$leaderboard", "user123", "TestUser", "channel123")

	bot.leaderboardHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
}

// endregion

// region setPredictions tests

func TestSetPredictions_Swiss_Success(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	// 10 unique valid teams: positions 0-1 → 3-0, 2-7 → Advance, 8-9 → 0-3
	message := createMockMessage(
		"$set \"Team A\" \"Team B\" \"Team C\" \"Team D\" \"Team E\" \"Team F\" \"Team G\" \"Team H\" \"Team I\" \"Team J\"",
		"user123", "TestUser", "channel123",
	)

	bot.setPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentEmbeds, 1)
	embed := mockSession.GetLastEmbed().Embed
	assert.Equal(t, "channel123", mockSession.GetLastEmbed().ChannelID)
	assert.Equal(t, "Pick'Ems Updated", embed.Title)
	assert.Contains(t, embed.Description, "TestUser")
}

func TestSetPredictions_Swiss_EmbedFieldsMatchPrediction(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage(
		"$set \"Team A\" \"Team B\" \"Team C\" \"Team D\" \"Team E\" \"Team F\" \"Team G\" \"Team H\" \"Team I\" \"Team J\"",
		"user123", "TestUser", "channel123",
	)

	bot.setPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentEmbeds, 1)
	fields := mockSession.GetLastEmbed().Embed.Fields
	require.Len(t, fields, 3)
	assert.Equal(t, "3-0", fields[0].Name)
	assert.Equal(t, "Team A, Team B", fields[0].Value)
	assert.Equal(t, "Advance", fields[1].Name)
	assert.Equal(t, "Team C, Team D, Team E, Team F, Team G, Team H", fields[1].Value)
	assert.Equal(t, "0-3", fields[2].Name)
	assert.Equal(t, "Team I, Team J", fields[2].Value)
}

func TestSetPredictions_SingleElim_EmbedFieldsMatchPrediction(t *testing.T) {
	// 8-team playoff store: ValidTeams must contain all 8 teams so
	// RequiredPredictions(8) = 4 passes the count check.
	mockStore := app.NewMockStore(tournament.SingleElim, "Playoffs")
	mockStore.SetEliminationResults(map[string]models.TeamProgress{
		"Team A": {Round: "Quarter Final", Status: "eliminated"},
		"Team B": {Round: "Quarter Final", Status: "eliminated"},
		"Team C": {Round: "Quarter Final", Status: "eliminated"},
		"Team D": {Round: "Quarter Final", Status: "eliminated"},
		"Team E": {Round: "Semi Final", Status: "eliminated"},
		"Team F": {Round: "Semi Final", Status: "eliminated"},
		"Team G": {Round: "Grand Final", Status: "eliminated"},
		"Team H": {Round: "Grand Final", Status: "advanced"},
	})
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3", EpochTime: 1700000000},
	})
	bot := &Bot{BotToken: "test_token", APIPtr: &app.App{Store: mockStore}}
	mockSession := NewMockDiscordSession()
	// 4 picks required (8 valid teams / 2); worst→best order so "Team D" becomes champion
	message := createMockMessage(
		"$set \"Team A\" \"Team B\" \"Team C\" \"Team D\"",
		"user123", "TestUser", "channel123",
	)

	bot.setPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentEmbeds, 1)
	fields := mockSession.GetLastEmbed().Embed.Fields
	require.GreaterOrEqual(t, len(fields), 3)
	assert.Equal(t, "Champion", fields[0].Name)
	assert.Equal(t, "Team D", fields[0].Value)
	assert.Equal(t, "Grand Final", fields[1].Name)
	assert.Equal(t, "Team C", fields[1].Value)
	assert.Equal(t, "Semi Final", fields[2].Name)
	assert.Contains(t, fields[2].Value, "Team A")
	assert.Contains(t, fields[2].Value, "Team B")
}

func TestPredictionFields_UnknownFormat(t *testing.T) {
	_, err := predictionFields(models.Prediction{Format: "unknown-format"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown prediction format")
}

func TestPredictionFields_SwissMalformed(t *testing.T) {
	_, err := predictionFields(models.Prediction{
		Format: string(tournament.Swiss),
		Win:    []string{"Team A"},
		Progression: map[string]models.TeamProgress{
			"Team B": {Round: "Grand Final", Status: "advanced"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected progression data")
}

func TestPredictionFields_SingleElimMalformed(t *testing.T) {
	_, err := predictionFields(models.Prediction{
		Format: string(tournament.SingleElim),
		Win:    []string{"Team A"},
		Progression: map[string]models.TeamProgress{
			"Team B": {Round: "Grand Final", Status: "advanced"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected swiss data")
}

func TestSetPredictions_InvalidInput(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	// Too few teams
	message := createMockMessage("$set Team1 Team2", "user123", "TestUser", "channel123")

	bot.setPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "error")
}

// endregion

// region checkPredictions tests

func TestCheckPredictions_NoPredictions(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$check", "user123", "TestUser", "channel123")

	bot.checkPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
	// Should indicate no predictions stored
	assert.Contains(t, strings.ToLower(msg.Content), "error")
}

func TestCheckPredictions_WithPredictions(t *testing.T) {
	// Create bot with a prediction already stored
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B"},
	})
	mockStore.SetSwissResults(map[string]string{
		"Team A": "3-0",
		"Team B": "3-1",
		"Team C": "3-2",
		"Team D": "2-3",
		"Team E": "1-3",
		"Team F": "0-3",
	})

	// Store a prediction for the user
	mockStore.StoreUserPrediction("user123", models.Prediction{
		UserID:   "user123",
		Username: "TestUser",
		Format:   "swiss",
		Round:    "test_round",
		Win:      []string{"Team A", "Team B"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team A", "Team B"},
		Lose:     []string{"Team E", "Team F"},
	})

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$check", "user123", "TestUser", "channel123")

	bot.checkPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
}

// endregion

// region newMessage routing tests

func TestNewMessage_IgnoresBotMessages(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()

	// Create a message from the bot itself
	message := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content:   "$help",
			ChannelID: "channel123",
			Author: &discordgo.User{
				ID:       "bot_user_id",
				Username: "PickemsBot",
			},
		},
	}

	// Simulate the bot's user ID matching the message author
	bot.newMessageHandler(mockSession, message, "bot_user_id")

	// Should not send any message since it's from the bot itself
	assert.Len(t, mockSession.SentMessages, 0)
}

func TestNewMessage_RoutesHelpCommand(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$help", "user123", "TestUser", "channel123")

	bot.newMessageHandler(mockSession, message, "bot_id")

	require.Len(t, mockSession.SentMessages, 1)
	assert.Contains(t, mockSession.GetLastMessage().Content, "PickEms Bot")
}

func TestNewMessage_RoutesTeamsCommand(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$teams", "user123", "TestUser", "channel123")

	bot.newMessageHandler(mockSession, message, "bot_id")

	require.Len(t, mockSession.SentMessages, 1)
	assert.Contains(t, mockSession.GetLastMessage().Content, "Teams in this Stage")
}

func TestNewMessage_IgnoresUnknownCommands(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("hello world", "user123", "TestUser", "channel123")

	bot.newMessageHandler(mockSession, message, "bot_id")

	// Should not respond to non-command messages
	assert.Len(t, mockSession.SentMessages, 0)
}

// endregion

// region additional newMessage routing tests

func TestNewMessage_RoutesDetailsCommand(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$details", "user123", "TestUser", "channel123")

	bot.newMessageHandler(mockSession, message, "bot_id")

	require.Len(t, mockSession.SentMessages, 1)
}

func TestNewMessage_RoutesSetCommand(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$set Team1", "user123", "TestUser", "channel123")

	bot.newMessageHandler(mockSession, message, "bot_id")

	require.Len(t, mockSession.SentMessages, 1)
}

func TestNewMessage_RoutesCheckCommand(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$check", "user123", "TestUser", "channel123")

	bot.newMessageHandler(mockSession, message, "bot_id")

	require.Len(t, mockSession.SentMessages, 1)
}

func TestNewMessage_RoutesLeaderboardCommand(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$leaderboard", "user123", "TestUser", "channel123")

	bot.newMessageHandler(mockSession, message, "bot_id")

	require.Len(t, mockSession.SentMessages, 1)
}

func TestNewMessage_RoutesUpcomingCommand(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$upcoming", "user123", "TestUser", "channel123")

	bot.newMessageHandler(mockSession, message, "bot_id")

	require.Len(t, mockSession.SentMessages, 1)
}

// endregion

// region leaderboard error tests

func TestLeaderboard_APIError(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.FetchLeaderboardFromDBError = errors.New("database error")

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$leaderboard", "user123", "TestUser", "channel123")

	bot.leaderboardHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "error")
}

// endregion

// region error handling tests

func TestTeams_APIError(t *testing.T) {
	// Create a mock store that will return an error
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.GetValidTeamsError = errors.New("database error")

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$teams", "user123", "TestUser", "channel123")

	bot.teamsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "error")
}

func TestUpcomingMatches_APIError(t *testing.T) {
	// Create a mock store that will return an error for schedule
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})
	// Set error for schedule fetch
	mockStore.SetScheduleError(errors.New("database error"))

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$upcoming", "user123", "TestUser", "channel123")

	bot.upcomingMatchesHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "error")
}

func TestUpcomingMatches_WithConfirmedMatches(t *testing.T) {
	// Create bot with matches that have confirmed teams (no TBD)
	// Use future epoch time so matches are considered "upcoming"
	futureTime := int64(9999999999)
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Navi", Team2: "G2", BestOf: "3", EpochTime: futureTime},
		{Team1: "Faze", Team2: "Vitality", BestOf: "3", EpochTime: futureTime + 1000},
	})
	mockStore.SetSwissResults(map[string]string{"Navi": "2-0", "G2": "1-1"})

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$upcoming", "user123", "TestUser", "channel123")

	bot.upcomingMatchesHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	// The handler should return upcoming matches or "No upcoming matches" depending on implementation
	assert.NotEmpty(t, msg.Content)
}

func TestDetails_APIError(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.GetValidTeamsError = errors.New("database error")

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$details", "user123", "TestUser", "channel123")

	bot.detailsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "error")
}

func TestCheckPredictions_ByUsername_Found(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B"},
	})
	mockStore.SetSwissResults(map[string]string{
		"Team A": "3-0",
		"Team B": "3-1",
	})
	mockStore.StoreUserPrediction("user456", models.Prediction{
		UserID:   "user456",
		Username: "PickemsBot",
		Format:   "swiss",
		Round:    "test_round",
		Win:      []string{"Team A", "Team B"},
		Advance:  []string{"Team C", "Team D", "Team E", "Team F", "Team A", "Team B"},
		Lose:     []string{"Team E", "Team F"},
	})

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$check PickemsBot", "user123", "TestUser", "channel123")

	bot.checkPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	embed := mockSession.GetLastEmbed()
	require.NotNil(t, embed)
	assert.Equal(t, "channel123", embed.ChannelID)
	assert.Contains(t, embed.Embed.Title, "PickemsBot")
}

func TestCheckPredictions_ByUsername_NotFound(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$check ghost", "user123", "TestUser", "channel123")

	bot.checkPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "no pick'ems found")
}

func TestCheckPredictions_GenericError(t *testing.T) {
	mockStore := app.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B"},
	})
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})
	// Set a generic error (not mongo.ErrNoDocuments)
	mockStore.GetUserPredictionError = errors.New("database connection failed")

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &app.App{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$check", "user123", "TestUser", "channel123")

	bot.checkPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "error")
}

// endregion

// region results tests

func TestResults_Success(t *testing.T) {
	// Set up a self-contained temp dir with a dummy result.png so the handler
	// can open it without depending on a pre-generated file on disk.
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "resources"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "resources", "result.png"), []byte("dummy png"), 0644))
	t.Chdir(tmpDir)

	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$results", "user123", "TestUser", "channel123")

	bot.resultsHandler(mockSession, message)

	require.Len(t, mockSession.SentFiles, 1)
	assert.Equal(t, "channel123", mockSession.SentFiles[0].ChannelID)
	assert.Equal(t, "resources/result.png", mockSession.SentFiles[0].Name)
}

func TestResults_FileNotFound(t *testing.T) {
	// No chdir — bot/ has no resources/result.png, so os.Open will fail
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	message := createMockMessage("$results", "user123", "TestUser", "channel123")

	bot.resultsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	assert.Contains(t, strings.ToLower(mockSession.SentMessages[0].Content), "error")
	assert.Len(t, mockSession.SentFiles, 0)
}

// endregion

// region mock session tests

func TestMockSession_ErrorToReturn(t *testing.T) {
	mockSession := NewMockDiscordSession()
	mockSession.ErrorToReturn = errors.New("send failed")

	_, err := mockSession.ChannelMessageSend("channel123", "test message")

	assert.Error(t, err)
	assert.Equal(t, "send failed", err.Error())
	// No messages should be stored when error is returned
	assert.Len(t, mockSession.SentMessages, 0)
}

func TestMockSession_GetLastMessage_Empty(t *testing.T) {
	mockSession := NewMockDiscordSession()

	msg := mockSession.GetLastMessage()

	assert.Empty(t, msg.ChannelID)
	assert.Empty(t, msg.Content)
}

func TestMockSession_ClearMessages(t *testing.T) {
	mockSession := NewMockDiscordSession()
	mockSession.ChannelMessageSend("channel1", "message1")
	mockSession.ChannelMessageSend("channel2", "message2")

	assert.Len(t, mockSession.SentMessages, 2)

	mockSession.ClearMessages()

	assert.Len(t, mockSession.SentMessages, 0)
}

// endregion
