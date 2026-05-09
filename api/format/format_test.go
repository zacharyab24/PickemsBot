/* format_test.go
 * Tests for the package registry.
 */

package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet_ReturnsRegisteredFormats(t *testing.T) {
	swiss, err := Get(Swiss)
	assert.NoError(t, err)
	assert.Equal(t, Swiss, swiss.Name())

	se, err := Get(SingleElim)
	assert.NoError(t, err)
	assert.Equal(t, SingleElim, se.Name())
}

func TestGet_UnknownReturnsError(t *testing.T) {
	_, err := Get("does-not-exist")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}

func TestMustGet_ReturnsRegisteredFormat(t *testing.T) {
	assert.Equal(t, Swiss, MustGet(Swiss).Name())
}

func TestMustGet_PanicsOnUnknown(t *testing.T) {
	assert.Panics(t, func() { MustGet("does-not-exist") })
}

func TestNames_ContainsRegisteredFormats(t *testing.T) {
	names := Names()
	assert.Contains(t, names, Swiss)
	assert.Contains(t, names, SingleElim)
}

func TestRegister_PanicsOnDuplicate(t *testing.T) {
	assert.Panics(t, func() { register(swissFormat{}) })
}
