//go:build integration

package store

import (
	"context"
	"testing"

	"pickems-bot/sources"
	"pickems-bot/tournament"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFetcher is a DataSourceFetcher that returns pre-configured data.
type mockFetcher struct {
	result tournament.MatchResult
	nodes  []sources.MatchNode
	err    error
}

func (m mockFetcher) FetchMatchData(round string) (tournament.MatchResult, []sources.MatchNode, error) {
	return m.result, m.nodes, m.err
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
	s := newTestStoreWithFetcher(t, mockFetcher{result: result, nodes: nodes})

	tournamentID := seedTournamentNullFormat(t, "test-fetch-writes")
	require.NoError(t, s.FetchAndSaveMatchResults(ctx, tournamentID, "Stage 1"))

	var count int
	require.NoError(t, testPool.QueryRow(ctx, `SELECT COUNT(*) FROM matches WHERE tournament_id = $1`, tournamentID).Scan(&count))
	assert.Equal(t, 2, count)
}

func TestFetchAndSaveMatchResults_FetcherError(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	s := newTestStoreWithFetcher(t, mockFetcher{err: assert.AnError})
	tournamentID := seedTournamentNullFormat(t, "test-fetch-error")

	err := s.FetchAndSaveMatchResults(ctx, tournamentID, "Stage 1")
	assert.Error(t, err)
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
