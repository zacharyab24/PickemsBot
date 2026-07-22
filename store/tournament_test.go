//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTournaments_ReturnsAllOrderedByName(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	// seedTournament inserts source="test", external_id=name.
	zebraID := seedTournament(t, "Zebra Cup", "swiss")
	alphaID := seedTournament(t, "Alpha Major", "single-elimination")

	got, err := s.ListTournaments(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Ordered by name, so Alpha before Zebra.
	assert.Equal(t, alphaID, got[0].ID)
	assert.Equal(t, "Alpha Major", got[0].Name)
	assert.Equal(t, zebraID, got[1].ID)
	assert.Equal(t, "Zebra Cup", got[1].Name)
}

func TestListTournaments_Empty(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	got, err := s.ListTournaments(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}
