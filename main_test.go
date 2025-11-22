/* main_test.go
 * Contains unit tests for main.go functions
 * Authors: Zachary Bower
 */

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConvertStrToBool_True tests converting "true" string
func TestConvertStrToBool_True(t *testing.T) {
	result, err := convertStrToBool("true")

	assert.NoError(t, err)
	assert.True(t, result)
}

// TestConvertStrToBool_False tests converting "false" string
func TestConvertStrToBool_False(t *testing.T) {
	result, err := convertStrToBool("false")

	assert.NoError(t, err)
	assert.False(t, result)
}

// TestConvertStrToBool_CaseInsensitiveTrue tests case-insensitive "TRUE"
func TestConvertStrToBool_CaseInsensitiveTrue(t *testing.T) {
	result, err := convertStrToBool("TRUE")

	assert.NoError(t, err)
	assert.True(t, result)
}

// TestConvertStrToBool_CaseInsensitiveFalse tests case-insensitive "FALSE"
func TestConvertStrToBool_CaseInsensitiveFalse(t *testing.T) {
	result, err := convertStrToBool("FALSE")

	assert.NoError(t, err)
	assert.False(t, result)
}

// TestConvertStrToBool_MixedCase tests mixed case "TrUe"
func TestConvertStrToBool_MixedCase(t *testing.T) {
	result, err := convertStrToBool("TrUe")

	assert.NoError(t, err)
	assert.True(t, result)
}

// TestConvertStrToBool_WithWhitespace tests string with leading/trailing whitespace
func TestConvertStrToBool_WithWhitespace(t *testing.T) {
	result, err := convertStrToBool("  true  ")

	assert.NoError(t, err)
	assert.True(t, result)
}

// TestConvertStrToBool_InvalidString tests invalid boolean string
func TestConvertStrToBool_InvalidString(t *testing.T) {
	_, err := convertStrToBool("yes")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid boolean string")
}

// TestConvertStrToBool_EmptyString tests empty string
func TestConvertStrToBool_EmptyString(t *testing.T) {
	_, err := convertStrToBool("")

	assert.Error(t, err)
}

// TestConvertStrToBool_NumberString tests numeric string
func TestConvertStrToBool_NumberString(t *testing.T) {
	_, err := convertStrToBool("1")

	assert.Error(t, err)
}

// TestConvertStrToBool_OnlyWhitespace tests string with only whitespace
func TestConvertStrToBool_OnlyWhitespace(t *testing.T) {
	_, err := convertStrToBool("   ")

	assert.Error(t, err)
}
