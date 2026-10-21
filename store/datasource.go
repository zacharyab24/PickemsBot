package store

import (
	"errors"

	"pickems-bot/sources"
	"pickems-bot/tournament"
)

// DataSourceFetcher interface — two implementations: Liquipedia, PandaScore
type DataSourceFetcher interface {
	// FetchMatchData fetches raw match data for round, plus a MatchResult
	// built from it when the format supports one. knownKind is the
	// tournament's already-persisted format; pass "" if not yet known -
	// otherwise it's trusted over re-deriving a kind from match node sections.
	// If the resolved kind doesn't support predictions (e.g.
	// double-elimination), result is nil but nodes and kind are still valid,
	// so raw match data can still be persisted and displayed.
	FetchMatchData(round string, knownKind tournament.Kind) (result tournament.MatchResult, nodes []sources.MatchNode, kind tournament.Kind, err error)
	FetchSchedule() ([]sources.ScheduledMatch, error)
}

// LiquipediaFetcher implements the DataSourceFetcher interface
type LiquipediaFetcher struct {
	apiURL string
	apiKey string
	page   string
}

// PandaScoreFetcher implements the DataSourceFetcher interface
type PandaScoreFetcher struct {
	apiURL       string
	apiKey       string
	seriesID     int
	tournamentID int
}

// buildMatchResult calls format.BuildFromMatchNodes and translates
// ErrPredictionsUnsupported into the "no result, but nodes/kind still valid"
// shape both FetchMatchData implementations below return.
func buildMatchResult(format tournament.Format, nodes []sources.MatchNode, round string, kind tournament.Kind) (tournament.MatchResult, []sources.MatchNode, tournament.Kind, error) {
	result, err := format.BuildFromMatchNodes(nodes, round)
	if errors.Is(err, tournament.ErrPredictionsUnsupported) {
		return nil, nodes, kind, nil
	}
	if err != nil {
		return nil, nil, "", err
	}
	return result, nodes, kind, nil
}

// NewLiquipediaFetcher creates a LiquipediaFetcher with the given API URL, API key and page path.
func NewLiquipediaFetcher(apiURL, apiKey, page string) LiquipediaFetcher {
	return LiquipediaFetcher{apiURL: apiURL, apiKey: apiKey, page: page}
}

// FetchMatchData fetches match data using liquipedia as a datasource, filtered to the current round of a tournament.
// See DataSourceFetcher.FetchMatchData for knownKind and the unsupported-format return shape.
func (f LiquipediaFetcher) FetchMatchData(round string, knownKind tournament.Kind) (tournament.MatchResult, []sources.MatchNode, tournament.Kind, error) {
	matchData, err := sources.GetLiquipediaMatchDataByPage(f.apiURL, f.apiKey, f.page)
	if err != nil {
		return nil, nil, "", err
	}

	matchNodes, err := sources.ParseLiquipediaMatches(matchData)
	if err != nil {
		return nil, nil, "", err
	}

	kind := knownKind
	if kind == "" {
		kind, err = tournament.DetectKindFromMatchNodes(matchNodes)
		if err != nil {
			return nil, nil, "", err
		}
	}

	format, err := tournament.Get(kind)
	if err != nil {
		return nil, nil, "", err
	}

	matchNodes = tournament.FilterNodesByKind(matchNodes, kind)

	return buildMatchResult(format, matchNodes, round, kind)
}

// FetchSchedule fetches the scheduled matches for the tournament using Liquipedia as a datasource.
// Doesn't do any filtering, callers are responsible for filtering by round / time / etc
func (f LiquipediaFetcher) FetchSchedule() ([]sources.ScheduledMatch, error) {
	matchData, err := sources.GetLiquipediaMatchDataByPage(f.apiURL, f.apiKey, f.page)
	if err != nil {
		return nil, err
	}
	return sources.ParseLiquipediaSchedule(matchData)
}

// NewPandaScoreFetcher creates a PandaScoreFetcher with the given API URL, API key, series ID, and optional tournament ID.
func NewPandaScoreFetcher(apiURL string, apiKey string, seriesID int, tournamentID int) PandaScoreFetcher {
	return PandaScoreFetcher{apiURL: apiURL, apiKey: apiKey, seriesID: seriesID, tournamentID: tournamentID}
}

// FetchMatchData fetches match data using PandaSource as a datasource, filtered to the current round of a tournament.
// See DataSourceFetcher.FetchMatchData for knownKind and the unsupported-format return shape.
func (f PandaScoreFetcher) FetchMatchData(round string, knownKind tournament.Kind) (tournament.MatchResult, []sources.MatchNode, tournament.Kind, error) {
	matchData, err := sources.GetPandaScoreMatches(f.apiURL, f.apiKey, f.seriesID, f.tournamentID)
	if err != nil {
		return nil, nil, "", err
	}

	matchNodes, err := sources.ParsePandaScoreMatches(matchData, f.tournamentID)
	if err != nil {
		return nil, nil, "", err
	}

	kind := knownKind
	if kind == "" {
		kind, err = tournament.DetectKindFromMatchNodes(matchNodes)
		if err != nil {
			return nil, nil, "", err
		}
	}

	format, err := tournament.Get(kind)
	if err != nil {
		return nil, nil, "", err
	}

	matchNodes = tournament.FilterNodesByKind(matchNodes, kind)

	return buildMatchResult(format, matchNodes, round, kind)
}

// FetchSchedule fetches the scheduled matches for the tournament using PandaScore as a datasource.
// Doesn't do any filtering, callers are responsible for filtering by round / time / etc
func (f PandaScoreFetcher) FetchSchedule() ([]sources.ScheduledMatch, error) {
	matchData, err := sources.GetPandaScoreMatches(f.apiURL, f.apiKey, f.seriesID, f.tournamentID)
	if err != nil {
		return nil, err
	}
	return sources.ParsePandaScoreSchedule(matchData, f.tournamentID)
}

// Compile time assertions to ensure interface impls have all methods defined
var _ DataSourceFetcher = (*LiquipediaFetcher)(nil)
var _ DataSourceFetcher = (*PandaScoreFetcher)(nil)
