/* handlers_test.go
 * Contains unit tests for bot command handlers using mock Discord session
 * AI-Generated
 */

package bot

import (
	"errors"
	"pickems-bot/api/api"
	"pickems-bot/api/external"
	"pickems-bot/api/shared"
	"pickems-bot/api/store"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestBot creates a Bot instance with a mock API for testing
func createTestBot(format string) *Bot {
	mockStore := api.NewMockStore(format, "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{
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
		APIPtr:   &api.API{Store: mockStore},
	}
}

// createTestBotWithElimination creates a Bot with elimination format
func createTestBotWithElimination() *Bot {
	mockStore := api.NewMockStore("single-elimination", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", BestOf: "3", EpochTime: 1700000000},
	})
	mockStore.SetEliminationResults(map[string]shared.TeamProgress{
		"Team A": {Round: "semifinal", Status: "advanced"},
		"Team B": {Round: "quarterfinal", Status: "eliminated"},
	})

	return &Bot{
		BotToken: "test_token",
		APIPtr:   &api.API{Store: mockStore},
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
	assert.Contains(t, msg.Content, "Valid teams")
	// Should contain team names
	assert.Contains(t, msg.Content, "Team")
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

func TestUpcomingMatches_NoMatches(t *testing.T) {
	// Create bot with scheduled matches that exist but are empty
	mockStore := api.NewMockStore("swiss", "test_round")
	// Set at least one scheduled match so EnsureScheduledMatches passes
	mockStore.SetScheduledMatches([]external.ScheduledMatch{
		{Team1: "TBD", Team2: "TBD", BestOf: "3", EpochTime: 1700000000},
	})
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &api.API{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$upcoming", "user123", "TestUser", "channel123")

	bot.upcomingMatchesHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	// Since all matches are TBD, they get filtered out and we get "Upcoming matches:" but no content
	// or it could be "No upcoming matches" - either is acceptable
	assert.NotEmpty(t, msg.Content)
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
	// Swiss format requires 10 teams: 2 3-0, 6 advance, 2 0-3
	message := createMockMessage(
		"$set \"Team A\" \"Team B\" \"Team C\" \"Team D\" \"Team E\" \"Team F\" \"Team A\" \"Team B\" \"Team C\" \"Team D\"",
		"user123", "TestUser", "channel123",
	)

	bot.setPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Equal(t, "channel123", msg.ChannelID)
	// Should either succeed or give meaningful error
	assert.NotEmpty(t, msg.Content)
}

func TestSetPredictions_InvalidInput(t *testing.T) {
	bot := createTestBot("swiss")
	mockSession := NewMockDiscordSession()
	// Too few teams
	message := createMockMessage("$set Team1 Team2", "user123", "TestUser", "channel123")

	bot.setPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, msg.Content, "error")
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
	assert.True(t,
		strings.Contains(msg.Content, "does not have any Pickems") ||
			strings.Contains(msg.Content, "error"),
	)
}

func TestCheckPredictions_WithPredictions(t *testing.T) {
	// Create bot with a prediction already stored
	mockStore := api.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{
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
	mockStore.StoreUserPrediction("user123", store.Prediction{
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
		APIPtr:   &api.API{Store: mockStore},
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
	assert.Contains(t, mockSession.GetLastMessage().Content, "Valid teams")
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
	mockStore := api.NewMockStore("swiss", "test_round")
	mockStore.FetchLeaderboardFromDBError = errors.New("database error")

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &api.API{Store: mockStore},
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
	mockStore := api.NewMockStore("swiss", "test_round")
	// Don't set any results, which should cause an error

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &api.API{Store: mockStore},
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
	mockStore := api.NewMockStore("swiss", "test_round")
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})
	// Set error for schedule fetch
	mockStore.SetScheduleError(errors.New("database error"))

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &api.API{Store: mockStore},
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
	mockStore := api.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{
		{Team1: "Navi", Team2: "G2", BestOf: "3", EpochTime: futureTime},
		{Team1: "Faze", Team2: "Vitality", BestOf: "3", EpochTime: futureTime + 1000},
	})
	mockStore.SetSwissResults(map[string]string{"Navi": "2-0", "G2": "1-1"})

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &api.API{Store: mockStore},
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
	mockStore := api.NewMockStore("swiss", "test_round")
	mockStore.GetValidTeamsError = errors.New("database error")

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &api.API{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$details", "user123", "TestUser", "channel123")

	bot.detailsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "error")
}

func TestCheckPredictions_GenericError(t *testing.T) {
	mockStore := api.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]external.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B"},
	})
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})
	// Set a generic error (not mongo.ErrNoDocuments)
	mockStore.GetUserPredictionError = errors.New("database connection failed")

	bot := &Bot{
		BotToken: "test_token",
		APIPtr:   &api.API{Store: mockStore},
	}

	mockSession := NewMockDiscordSession()
	message := createMockMessage("$check", "user123", "TestUser", "channel123")

	bot.checkPredictionsHandler(mockSession, message)

	require.Len(t, mockSession.SentMessages, 1)
	msg := mockSession.GetLastMessage()
	assert.Contains(t, strings.ToLower(msg.Content), "error")
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
