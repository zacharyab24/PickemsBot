//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"pickems-bot/sources"
	"pickems-bot/tournament"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFetcher is a DataSourceFetcher that returns pre-configured data.
type mockFetcher struct {
	result tournament.MatchResult
	nodes  []sources.MatchNode
	kind   tournament.Kind
	err    error
}

func (m mockFetcher) FetchMatchData(round string, knownKind tournament.Kind) (tournament.MatchResult, []sources.MatchNode, tournament.Kind, error) {
	return m.result, m.nodes, m.kind, m.err
}

func (m mockFetcher) FetchSchedule() ([]sources.ScheduledMatch, error) {
	return nil, nil
}

// region upsertMatchNodes

func TestUpsertMatchNodes_CreatesTeamRows(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-team-rows")

	nodes := []sources.MatchNode{
		{ID: "m1", Team1: "Navi", Team2: "FaZe", Status: "not_started"},
		{ID: "m2", Team1: "MOUZ", Team2: "Vitality", Status: "not_started"},
	}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Playoffs", nodes, tournament.Swiss))

	var count int
	err := testPool.QueryRow(ctx, `SELECT COUNT(*) FROM teams WHERE source = 'pandascore'`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestUpsertMatchNodes_SetsTeamFKs(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-team-fks")

	nodes := []sources.MatchNode{
		{ID: "m1", Team1: "Navi", Team2: "FaZe", Status: "not_started"},
	}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Playoffs", nodes, tournament.Swiss))

	var team1ID, team2ID *int
	err := testPool.QueryRow(ctx, `SELECT team1_id, team2_id FROM matches WHERE tournament_id = $1`, tournamentID).
		Scan(&team1ID, &team2ID)
	require.NoError(t, err)
	assert.NotNil(t, team1ID, "team1_id should be set")
	assert.NotNil(t, team2ID, "team2_id should be set")
}

func TestUpsertMatchNodes_SetsFormatLazily(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-lazy-format")

	var formatBefore *string
	require.NoError(t, testPool.QueryRow(ctx, `SELECT format FROM tournaments WHERE id = $1`, tournamentID).Scan(&formatBefore))
	assert.Nil(t, formatBefore, "format should start NULL")

	nodes := []sources.MatchNode{{ID: "m1", Team1: "TeamA", Team2: "TeamB", Status: "not_started"}}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.Swiss))

	var formatAfter string
	require.NoError(t, testPool.QueryRow(ctx, `SELECT format FROM tournaments WHERE id = $1`, tournamentID).Scan(&formatAfter))
	assert.Equal(t, "swiss", formatAfter)
}

func TestUpsertMatchNodes_DoesNotOverwriteExistingFormat(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-no-overwrite-format", "swiss")

	nodes := []sources.MatchNode{{ID: "m1", Team1: "TeamA", Team2: "TeamB", Status: "not_started"}}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.SingleElim))

	var format string
	require.NoError(t, testPool.QueryRow(ctx, `SELECT format FROM tournaments WHERE id = $1`, tournamentID).Scan(&format))
	assert.Equal(t, "swiss", format, "existing format should not be overwritten")
}

// TestUpsertMatchNodes_OtherDoesNotLockFormat verifies Other leaves format
// NULL so a later, better-classified fetch can still set it.
func TestUpsertMatchNodes_OtherDoesNotLockFormat(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-other-no-lock")

	nodes := []sources.MatchNode{{ID: "m1", Team1: "TeamA", Team2: "TeamB", Status: "not_started"}}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.Other))

	var format *string
	require.NoError(t, testPool.QueryRow(ctx, `SELECT format FROM tournaments WHERE id = $1`, tournamentID).Scan(&format))
	assert.Nil(t, format, "format should stay NULL after an ambiguous (Other) detection")

	// A later fetch with recognisable sections should still be able to set it.
	nodes[0].Section = "Round 1"
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.Swiss))
	require.NoError(t, testPool.QueryRow(ctx, `SELECT format FROM tournaments WHERE id = $1`, tournamentID).Scan(&format))
	require.NotNil(t, format)
	assert.Equal(t, "swiss", *format)
}

func TestUpsertMatchNodes_Idempotent(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-idempotent")

	nodes := []sources.MatchNode{
		{ID: "m1", Team1: "TeamA", Team2: "TeamB", Status: "not_started"},
	}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.Swiss))
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.Swiss))

	var matchCount int
	require.NoError(t, testPool.QueryRow(ctx, `SELECT COUNT(*) FROM matches WHERE tournament_id = $1`, tournamentID).Scan(&matchCount))
	assert.Equal(t, 1, matchCount)

	var teamCount int
	require.NoError(t, testPool.QueryRow(ctx, `SELECT COUNT(*) FROM teams WHERE source = 'pandascore'`).Scan(&teamCount))
	assert.Equal(t, 2, teamCount)
}

func TestUpsertMatchNodes_UpdatesCompletedMatch(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-completed-match")

	pending := []sources.MatchNode{{ID: "m1", Team1: "TeamA", Team2: "TeamB", Status: "not_started"}}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", pending, tournament.Swiss))

	finished := []sources.MatchNode{{ID: "m1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA", Score: "2-1", Status: "finished"}}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", finished, tournament.Swiss))

	var status, score string
	err := testPool.QueryRow(ctx, `SELECT status, COALESCE(score, '') FROM matches WHERE tournament_id = $1`, tournamentID).
		Scan(&status, &score)
	require.NoError(t, err)
	assert.Equal(t, "completed", status)
	assert.Equal(t, "2-1", score)
}

func TestUpsertMatchNodes_CompletedAt_NotBumpedOnReUpsert(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-completed-at-stable")

	finished := []sources.MatchNode{{ID: "m1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA", Score: "2-1", Status: "finished"}}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", finished, tournament.Swiss))

	var firstCompletedAt time.Time
	require.NoError(t, testPool.QueryRow(ctx, `SELECT completed_at FROM matches WHERE tournament_id = $1`, tournamentID).Scan(&firstCompletedAt))
	require.False(t, firstCompletedAt.IsZero())

	time.Sleep(10 * time.Millisecond)

	// A sibling match finishing in the same round re-upserts every node,
	// including this already-completed one - completed_at must not move.
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", finished, tournament.Swiss))

	var secondCompletedAt time.Time
	require.NoError(t, testPool.QueryRow(ctx, `SELECT completed_at FROM matches WHERE tournament_id = $1`, tournamentID).Scan(&secondCompletedAt))
	assert.True(t, firstCompletedAt.Equal(secondCompletedAt), "completed_at changed on re-upsert: %v -> %v", firstCompletedAt, secondCompletedAt)
}

func TestUpsertMatchNodes_WritesScheduledAtFromTimestamp(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournamentNullFormat(t, "test-scheduled-at")

	nodes := []sources.MatchNode{
		{ID: "m1", Team1: "TeamA", Team2: "TeamB", Status: "not_started", Timestamp: 1763215200},
	}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.Swiss))

	var scheduledAt time.Time
	err := testPool.QueryRow(ctx, `SELECT scheduled_at FROM matches WHERE tournament_id = $1`, tournamentID).Scan(&scheduledAt)
	require.NoError(t, err)
	assert.Equal(t, int64(1763215200), scheduledAt.Unix())
}

// endregion

// region FetchAndSaveMatchResults

func TestFetchAndSaveMatchResults_WritesNodes(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	nodes := []sources.MatchNode{
		{ID: "m1", Team1: "TeamA", Team2: "TeamB", Status: "not_started"},
		{ID: "m2", Team1: "TeamC", Team2: "TeamD", Status: "not_started"},
	}
	result := tournament.SwissResult{Round: "Stage 1", Teams: map[string]string{}}
	s := newTestStoreWithFetcher(t, mockFetcher{result: result, nodes: nodes, kind: tournament.Swiss})

	tournamentID := seedTournamentNullFormat(t, "test-fetch-writes")
	require.NoError(t, s.FetchAndSaveMatchResults(ctx, tournamentID, "Stage 1"))

	var count int
	require.NoError(t, testPool.QueryRow(ctx, `SELECT COUNT(*) FROM matches WHERE tournament_id = $1`, tournamentID).Scan(&count))
	assert.Equal(t, 2, count)
}

// TestFetchAndSaveMatchResults_UnsupportedFormat_StillWritesNodes verifies raw
// match nodes are persisted even when the format has no scoreable MatchResult.
func TestFetchAndSaveMatchResults_UnsupportedFormat_StillWritesNodes(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	nodes := []sources.MatchNode{
		{ID: "m1", Team1: "TeamA", Team2: "TeamB", Status: "not_started"},
	}
	s := newTestStoreWithFetcher(t, mockFetcher{result: nil, nodes: nodes, kind: tournament.DoubleElim})

	tournamentID := seedTournamentNullFormat(t, "test-fetch-unsupported")
	require.NoError(t, s.FetchAndSaveMatchResults(ctx, tournamentID, "Group B"))

	var count int
	require.NoError(t, testPool.QueryRow(ctx, `SELECT COUNT(*) FROM matches WHERE tournament_id = $1`, tournamentID).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestFetchAndSaveMatchResults_FetcherError(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	s := newTestStoreWithFetcher(t, mockFetcher{err: assert.AnError})
	tournamentID := seedTournamentNullFormat(t, "test-fetch-error")

	err := s.FetchAndSaveMatchResults(ctx, tournamentID, "Stage 1")
	assert.Error(t, err)
}

// identityEchoFetcher returns a single match node whose external id mirrors
// whatever tournament it was resolved for, so a test can tell which
// tournament's identity actually drove the fetch.
type identityEchoFetcher struct {
	externalID string
}

func (f identityEchoFetcher) FetchMatchData(round string, knownKind tournament.Kind) (tournament.MatchResult, []sources.MatchNode, tournament.Kind, error) {
	nodes := []sources.MatchNode{{ID: f.externalID, Team1: "A", Team2: "B", Status: "not_started"}}
	return tournament.SwissResult{Round: round, Teams: map[string]string{}}, nodes, tournament.Swiss, nil
}

func (f identityEchoFetcher) FetchSchedule() ([]sources.ScheduledMatch, error) {
	return nil, nil
}

// TestFetchAndSaveMatchResults_UsesEachTournamentsOwnIdentity verifies each
// tournament's fetch uses its own external id, not another tournament's.
func TestFetchAndSaveMatchResults_UsesEachTournamentsOwnIdentity(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	resolve := func(t Tournament) (DataSourceFetcher, error) {
		return identityEchoFetcher{externalID: t.ExternalID}, nil
	}
	s := newTestStoreWithFetcherResolver(t, resolve)

	tournamentA := seedTournamentNullFormat(t, "tournament-a")
	tournamentB := seedTournamentNullFormat(t, "tournament-b")

	require.NoError(t, s.FetchAndSaveMatchResults(ctx, tournamentA, "Stage 1"))
	require.NoError(t, s.FetchAndSaveMatchResults(ctx, tournamentB, "Stage 1"))

	var externalIDA, externalIDB string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT external_id FROM matches WHERE tournament_id = $1`, tournamentA).Scan(&externalIDA))
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT external_id FROM matches WHERE tournament_id = $1`, tournamentB).Scan(&externalIDB))

	assert.Equal(t, "tournament-a", externalIDA, "tournament A's fetch should have used its own external id")
	assert.Equal(t, "tournament-b", externalIDB, "tournament B's fetch should have used its own external id")
}

// endregion

// region GetMatchResults

func TestGetMatchResults_Swiss(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-get-results-swiss", "swiss")
	nodes := []sources.MatchNode{
		{ID: "m1", Team1: "TeamA", Team2: "TeamB", Winner: "TeamA", Score: "3-0", Status: "finished"},
		{ID: "m2", Team1: "TeamC", Team2: "TeamD", Status: "not_started"},
	}
	require.NoError(t, s.upsertMatchNodes(ctx, tournamentID, "Stage 1", nodes, tournament.Swiss))

	result, err := s.GetMatchResults(ctx, tournamentID, "Stage 1")
	require.NoError(t, err)
	assert.Equal(t, tournament.Swiss, result.GetType())
}

func TestGetMatchResults_NoNodes(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := newTestStore(t)

	tournamentID := seedTournament(t, "test-get-results-empty", "swiss")

	_, err := s.GetMatchResults(ctx, tournamentID, "Stage 1")
	assert.Error(t, err)
}

// endregion
