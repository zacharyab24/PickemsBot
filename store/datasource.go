package store

import (
	"pickems-bot/sources"
	"pickems-bot/tournament"
)

// DataSourceFetcher interface — two implementations: Liquipedia, PandaScore
type DataSourceFetcher interface {
	// FetchMatchData fetches raw match data and builds a MatchResult for round.
	// knownKind is the tournament's already-persisted format (from
	// checkAndStoreFormat's bracket-based detection); pass "" if it isn't
	// known yet. When set, it's trusted over re-deriving a kind from the
	// fetched match nodes' Section strings - that keyword-based detection
	// (DetectKindFromMatchNodes) doesn't recognise every bracket shape a
	// tournament can take (e.g. group-stage/double-elimination naming
	// conventions), so re-guessing it here can disagree with, and be less
	// reliable than, the bracket-endpoint detection already done and stored.
	FetchMatchData(round string, knownKind tournament.Kind) (tournament.MatchResult, []sources.MatchNode, error)
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

// NewLiquipediaFetcher creates a LiquipediaFetcher with the given API URL, API key and page path.
func NewLiquipediaFetcher(apiURL, apiKey, page string) LiquipediaFetcher {
	return LiquipediaFetcher{apiURL: apiURL, apiKey: apiKey, page: page}
}

// FetchMatchData fetches match data using liquipedia as a datasource, filtered to the current round of a tournament.
// See DataSourceFetcher.FetchMatchData for knownKind.
func (f LiquipediaFetcher) FetchMatchData(round string, knownKind tournament.Kind) (tournament.MatchResult, []sources.MatchNode, error) {
	matchData, err := sources.GetLiquipediaMatchDataByPage(f.apiURL, f.apiKey, f.page)
	if err != nil {
		return nil, nil, err
	}

	matchNodes, err := sources.ParseLiquipediaMatches(matchData)
	if err != nil {
		return nil, nil, err
	}

	kind := knownKind
	if kind == "" {
		kind, err = tournament.DetectKindFromMatchNodes(matchNodes)
		if err != nil {
			return nil, nil, err
		}
	}

	format, err := tournament.Get(kind)
	if err != nil {
		return nil, nil, err
	}

	matchNodes = tournament.FilterNodesByKind(matchNodes, kind)

	result, err := format.BuildFromMatchNodes(matchNodes, round)
	if err != nil {
		return nil, nil, err
	}

	return result, matchNodes, nil
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
// See DataSourceFetcher.FetchMatchData for knownKind.
func (f PandaScoreFetcher) FetchMatchData(round string, knownKind tournament.Kind) (tournament.MatchResult, []sources.MatchNode, error) {
	matchData, err := sources.GetPandaScoreMatches(f.apiURL, f.apiKey, f.seriesID, f.tournamentID)
	if err != nil {
		return nil, nil, err
	}

	matchNodes, err := sources.ParsePandaScoreMatches(matchData, f.tournamentID)
	if err != nil {
		return nil, nil, err
	}

	kind := knownKind
	if kind == "" {
		kind, err = tournament.DetectKindFromMatchNodes(matchNodes)
		if err != nil {
			return nil, nil, err
		}
	}

	format, err := tournament.Get(kind)
	if err != nil {
		return nil, nil, err
	}

	matchNodes = tournament.FilterNodesByKind(matchNodes, kind)

	result, err := format.BuildFromMatchNodes(matchNodes, round)
	if err != nil {
		return nil, nil, err
	}

	return result, matchNodes, nil
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
