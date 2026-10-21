//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_EmptyConnString(t *testing.T) {
	_, err := NewStore("", nil, nil)
	assert.Error(t, err)
}

func TestNewStore_PingsAndCloses(t *testing.T) {
	s, err := NewStore(testConnStr, nil, nil)
	require.NoError(t, err)

	require.NoError(t, s.Ping(context.Background()))
	s.Close()
	assert.Error(t, s.Ping(context.Background()), "a closed store should not ping")
}
