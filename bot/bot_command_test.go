/* bot_command_test.go
 * Contains unit tests for bot.go
 * Authors: Zachary Bower
 */

package bot

import (
	"strings"
	"testing"

	"pickems-bot/app"
	"pickems-bot/sources"
	"pickems-bot/tournament"
)

// Create a mock API for testing
func createMockAPI(kind tournament.Kind) *app.App {
	mockStore := app.NewMockStore(kind, "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B"}})
	mockStore.SetSwissResults(map[string]string{
		"Team A": "3-0",
		"Team B": "3-1",
	})
	return &app.App{Store: mockStore}
}

// region NewBot tests

func TestNewBot_Success(t *testing.T) {
	apiPtr := createMockAPI("swiss")
	bot, err := NewBot("test_token", apiPtr)

	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if bot.BotToken != "test_token" {
		t.Errorf("Expected bot token 'test_token', got '%s'", bot.BotToken)
	}

	if bot.APIPtr != apiPtr {
		t.Error("API pointer not set correctly")
	}
}

func TestNewBot_EmptyToken(t *testing.T) {
	apiPtr := createMockAPI("swiss")
	_, err := NewBot("", apiPtr)

	if err == nil {
		t.Error("Expected error for empty bot token, got nil")
	}

	if !strings.Contains(err.Error(), "botToken is required") {
		t.Errorf("Expected error about botToken, got: %s", err.Error())
	}
}

// endregion
