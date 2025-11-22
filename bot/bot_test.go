/* bot_test.go
 * Contains unit tests for bot.go functions
 * Authors: Zachary Bower
 */

package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStartsWith_ExactMatch tests when input exactly matches the substring
func TestStartsWith_ExactMatch(t *testing.T) {
	result := startsWith("hello", "hello")
	assert.True(t, result)
}

// TestStartsWith_StartsWithSubstring tests when input starts with substring
func TestStartsWith_StartsWithSubstring(t *testing.T) {
	result := startsWith("hello world", "hello")
	assert.True(t, result)
}

// TestStartsWith_DoesNotStartWith tests when substring is present but not at start
func TestStartsWith_DoesNotStartWith(t *testing.T) {
	result := startsWith("world hello", "hello")
	assert.False(t, result)
}

// TestStartsWith_SubstringNotPresent tests when substring is not present at all
func TestStartsWith_SubstringNotPresent(t *testing.T) {
	result := startsWith("hello world", "goodbye")
	assert.False(t, result)
}

// TestStartsWith_EmptySubstring tests with empty substring
func TestStartsWith_EmptySubstring(t *testing.T) {
	result := startsWith("hello", "")
	assert.True(t, result) // Empty string starts every string
}

// TestStartsWith_EmptyInput tests with empty input string
func TestStartsWith_EmptyInput(t *testing.T) {
	result := startsWith("", "hello")
	assert.False(t, result)
}

// TestStartsWith_BothEmpty tests when both strings are empty
func TestStartsWith_BothEmpty(t *testing.T) {
	result := startsWith("", "")
	assert.True(t, result)
}

// TestStartsWith_DiscordCommand tests with Discord command prefix
func TestStartsWith_DiscordCommand(t *testing.T) {
	result := startsWith("$help", "$")
	assert.True(t, result)
}

// TestStartsWith_LongerSubstring tests when substring is longer than input
func TestStartsWith_LongerSubstring(t *testing.T) {
	result := startsWith("hi", "hello")
	assert.False(t, result)
}

// TestStartsWith_CaseSensitive tests that function is case-sensitive
func TestStartsWith_CaseSensitive(t *testing.T) {
	result := startsWith("Hello", "hello")
	assert.False(t, result)
}

// TestStartsWith_PartialMatch tests partial matching at the beginning
func TestStartsWith_PartialMatch(t *testing.T) {
	result := startsWith("$set team1 team2", "$set")
	assert.True(t, result)
}

// TestStartsWith_SpecialCharacters tests with special characters
func TestStartsWith_SpecialCharacters(t *testing.T) {
	result := startsWith("$check-predictions", "$check")
	assert.True(t, result)
}
