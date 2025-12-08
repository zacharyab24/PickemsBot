/* bot.go
 * Contains logic used for creating and running the bot. Requires a discord bot token, and ApiPtr both of which are
 * passed in from main.go
 * Authors: Zachary Bower
 */

package bot

import (
	"fmt"
	"pickems-bot/api/api"
	"strings"
)

// Bot represents a Discord bot instance with its token and API pointer
type Bot struct {
	BotToken string
	APIPtr   *api.API
}

// NewBot creates a new Bot instance with the provided token and API pointer
func NewBot(botToken string, apiPtr *api.API) (*Bot, error) {
	if botToken == "" {
		return nil, fmt.Errorf("botToken is required but none was provided")
	}

	return &Bot{
		BotToken: botToken,
		APIPtr:   apiPtr,
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
