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

func TestSyncTournaments_UpsertsActiveSet(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	require.NoError(t, s.SyncTournaments(ctx, []TournamentCatalogEntry{
		{ExternalID: "21474", Name: "BLAST Bounty Summer 2026", Round: "Qualifier", SeriesID: "10801"},
		{ExternalID: "21475", Name: "BLAST Bounty Summer 2026", Round: "Playoffs", SeriesID: "10801"},
	}))

	names, err := s.ListTournamentNames(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"BLAST Bounty Summer 2026"}, names) // collapsed to one name

	rounds, err := s.ListRoundsForTournament(ctx, "BLAST Bounty Summer 2026")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Qualifier", "Playoffs"}, rounds)
}

func TestSyncTournaments_MarksDroppedAsFinished(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	require.NoError(t, s.SyncTournaments(ctx, []TournamentCatalogEntry{
		{ExternalID: "1", Name: "Event A", Round: "Playoffs", SeriesID: "100"},
		{ExternalID: "2", Name: "Event B", Round: "Playoffs", SeriesID: "200"},
	}))

	// Event B drops out of the active set -> swept to finished; A stays active.
	require.NoError(t, s.SyncTournaments(ctx, []TournamentCatalogEntry{
		{ExternalID: "1", Name: "Event A", Round: "Playoffs", SeriesID: "100"},
	}))

	assert.False(t, tournamentFinished(t, ctx, "1"), "Event A still active")
	assert.True(t, tournamentFinished(t, ctx, "2"), "Event B dropped -> finished")
}

func TestSyncTournaments_ReactivatesReturningTournament(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	require.NoError(t, s.SyncTournaments(ctx, []TournamentCatalogEntry{
		{ExternalID: "1", Name: "Event A", Round: "Playoffs", SeriesID: "100"},
		{ExternalID: "2", Name: "Event B", Round: "Playoffs", SeriesID: "200"},
	}))
	require.NoError(t, s.SyncTournaments(ctx, []TournamentCatalogEntry{
		{ExternalID: "1", Name: "Event A", Round: "Playoffs", SeriesID: "100"},
	})) // B -> finished
	require.NoError(t, s.SyncTournaments(ctx, []TournamentCatalogEntry{
		{ExternalID: "1", Name: "Event A", Round: "Playoffs", SeriesID: "100"},
		{ExternalID: "2", Name: "Event B", Round: "Playoffs", SeriesID: "200"},
	})) // B reappears in the active set

	assert.False(t, tournamentFinished(t, ctx, "2"), "Event B reappeared -> reactivated")
}

func TestSyncTournaments_EmptyActiveError(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	assert.Error(t, s.SyncTournaments(ctx, nil))
}

func tournamentFinished(t *testing.T, ctx context.Context, externalID string) bool {
	t.Helper()
	var finished bool
	err := testPool.QueryRow(ctx,
		`SELECT is_finished FROM tournaments WHERE source='pandascore' AND external_id=$1`, externalID).Scan(&finished)
	require.NoError(t, err)
	return finished
}
