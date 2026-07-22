//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTournamentNames_DistinctOrderedByName(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	// Two rounds of the same tournament must collapse to one name in the picklist.
	seedTournamentWithRound(t, "z1", "Zebra Cup", "Playoffs")
	seedTournamentWithRound(t, "a1", "Alpha Major", "Qualifier")
	seedTournamentWithRound(t, "a2", "Alpha Major", "Playoffs")

	got, err := s.ListTournamentNames(ctx)
	require.NoError(t, err)
	// Distinct, ordered by name: Alpha before Zebra.
	assert.Equal(t, []string{"Alpha Major", "Zebra Cup"}, got)
}

func TestListTournamentNames_Empty(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	got, err := s.ListTournamentNames(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestListRoundsForTournament_ScopedAndOrdered(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	seedTournamentWithRound(t, "a1", "Alpha Major", "Qualifier")
	seedTournamentWithRound(t, "a2", "Alpha Major", "Playoffs")
	seedTournamentWithRound(t, "b1", "Beta Cup", "Group Stage")

	got, err := s.ListRoundsForTournament(ctx, "Alpha Major")
	require.NoError(t, err)
	assert.Equal(t, []string{"Playoffs", "Qualifier"}, got) // ordered by round
}

func TestListRoundsForTournament_SkipsEmptyRounds(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	// A round-less row (round IS NULL) contributes nothing to the round picklist.
	seedTournament(t, "Alpha Major", "swiss")

	got, err := s.ListRoundsForTournament(ctx, "Alpha Major")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGetTournamentByNameAndRound_ResolvesRow(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	seedTournamentWithRound(t, "a1", "Alpha Major", "Qualifier")
	playoffsID := seedTournamentWithRound(t, "a2", "Alpha Major", "Playoffs")

	got, err := s.GetTournamentByNameAndRound(ctx, "Alpha Major", "Playoffs")
	require.NoError(t, err)
	assert.Equal(t, playoffsID, got.ID)
	assert.Equal(t, "Alpha Major", got.Name)
	assert.Equal(t, "Playoffs", got.Round)
}

func TestGetTournamentByNameAndRound_NoMatch(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	seedTournamentWithRound(t, "a1", "Alpha Major", "Qualifier")

	_, err := s.GetTournamentByNameAndRound(ctx, "Alpha Major", "Grand Final")
	assert.Error(t, err)
}

func TestGetTournament_ByID(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	id := seedTournamentWithRound(t, "a1", "Alpha Major", "Qualifier")

	got, err := s.GetTournament(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Alpha Major", got.Name)
	assert.Equal(t, "Qualifier", got.Round)
}
