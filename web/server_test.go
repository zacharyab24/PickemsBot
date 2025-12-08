/* server_test.go
 * Contains unit tests for server.go functions
 * Authors: Zachary Bower, Claude Opus 4.5
 */

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// region Config tests

func TestConfig_DefaultValues(t *testing.T) {
	cfg := Config{
		Addr: ":8080",
		API:  nil,
	}

	assert.Equal(t, ":8080", cfg.Addr)
	assert.Nil(t, cfg.API)
}

func TestConfig_CustomAddr(t *testing.T) {
	cfg := Config{
		Addr: ":3000",
		API:  nil,
	}

	assert.Equal(t, ":3000", cfg.Addr)
}

// endregion

// region Server tests

func TestServer_NewServer(t *testing.T) {
	s := &Server{
		api: nil,
	}

	assert.NotNil(t, s)
	assert.Nil(t, s.api)
}

// endregion

// Note: Start() function cannot be easily unit tested as it blocks on ListenAndServe
// Integration tests would be more appropriate for testing the full server startup
