/* bot.go
 * Contains logic used for creating and running the bot. Requires a discord bot token, and ApiPtr both of which are
 * passed in from main.go
 * Authors: Zachary Bower
 */

package bot

import (
	"fmt"
	"log/slog"
	"pickems-bot/app"
	"strings"
)

// Bot represents a Discord bot instance with its token and API pointer
type Bot struct {
	BotToken string
	APIPtr   *app.App
	log      *slog.Logger
}

// logger returns the bot's logger, falling back to the global default when none was injected.
func (b *Bot) logger() *slog.Logger {
	if b.log == nil {
		return slog.Default()
	}
	return b.log
}

// NewBot creates a new Bot instance with the provided token and API pointer.
// log may be nil; if so the global slog default is used.
func NewBot(botToken string, apiPtr *app.App, log *slog.Logger) (*Bot, error) {
	if botToken == "" {
		return nil, fmt.Errorf("botToken is required but none was provided")
	}

	var botLog *slog.Logger
	if log != nil {
		botLog = log.With("component", "bot")
	}
	return &Bot{
		BotToken: botToken,
		APIPtr:   apiPtr,
		log:      botLog,
	}, nil
}

// Helper function to check if a string starts with a given substring
// Preconditions: Receives an input string and a substring
// Postconditions: Returns true if the substring is at the start of the string, else returns false
func startsWith(inputString string, substring string) bool {
	//Check if the substring is present in the input string
	if !strings.Contains(inputString, substring) {
		return false
	}
	strLength := len(substring)
	for i := range strLength {
		if inputString[i] != substring[i] {
			return false
		}
	}
	return true
}
