package store

import (
	"context"
	"fmt"

	"pickems-bot/sources"
	"pickems-bot/tournament"
)

// GetMatchNodes retrieves the raw match nodes for a tournament round, used for results display and score calculation.
// Nodes are written to the matches table as part of FetchAndSaveMatchResults.
func (s *PostgresStore) GetMatchNodes(ctx context.Context, tournamentID int, round string) ([]sources.MatchNode, tournament.Kind, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.external_id, m.team1_name, m.team2_name,
		       COALESCE(tw.canonical_name, ''),
		       COALESCE(m.score, ''),
		       m.round,
		       m.status,
		       COALESCE(t.format, '')
		FROM matches m
		JOIN tournaments t ON t.id = m.tournament_id
		LEFT JOIN teams tw ON tw.id = m.winner_id
		WHERE m.tournament_id = $1 AND m.round = $2
		ORDER BY m.id ASC
	`, tournamentID, round)
	if err != nil {
		return nil, "", fmt.Errorf("GetMatchNodes: %w", err)
	}
	defer rows.Close()

	var nodes []sources.MatchNode
	var kind tournament.Kind
	for rows.Next() {
		var n sources.MatchNode
		var externalID *string
		if err := rows.Scan(&externalID, &n.Team1, &n.Team2, &n.Winner, &n.Score, &n.Section, &n.Status, &kind); err != nil {
			return nil, "", fmt.Errorf("GetMatchNodes: scan: %w", err)
		}
		if externalID != nil {
			n.ID = *externalID
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("GetMatchNodes: rows: %w", err)
	}
	if len(nodes) == 0 {
		return nil, "", fmt.Errorf("no match nodes found for tournament %d round %q", tournamentID, round)
	}
	return nodes, kind, nil
}