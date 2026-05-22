// Package format provides the per-tournament-format strategy interface
// (Swiss, single-elimination, etc.) plus the registry that callers use to
// dispatch by format name. Concrete formats live in their own files in this
// package and self-register via init().
package format

import (
	"fmt"
	"regexp"
	"strings"

	"pickems-bot/api/external"
	"pickems-bot/api/shared"
)

// Kind is the canonical string identifier for a tournament format.
// Values are persisted in MongoDB and surfaced in API/wikitext, so changing
// these strings is a breaking change.
type Kind string

// Known tournament formats. Add new formats by declaring a const here and
// registering an implementation from a new file (e.g. double_elimination.go).
const (
	// Swiss tournament format.
	Swiss Kind = "swiss"
	// SingleElim is a single-elimination bracket format.
	SingleElim Kind = "single-elimination"
	// DoubleElim is not fully supported yet. Only exists for upcoming only mode
	DoubleElim Kind = "double-elimination"
)

// MatchResult is the unified interface implemented by every per-format result
// type (SwissResult, EliminationResult, ...). The same value is used for
// in-memory scoring and on-disk persistence — the BSON tags on the concrete
// types define the storage layout.
type MatchResult interface {
	GetType() Kind
	GetRound() string
	// GetTeamNames returns just the team-name keys from the result. Useful for
	// callers that need the roster without caring about per-format value shape.
	GetTeamNames() []string
}

// ScoreReport is the structured result of CalculateScore. Callers that need
// format-specific data (e.g. to build a Discord embed) do a type switch on
// the concrete type (SwissReport, SingleElimReport, …).
type ScoreReport interface {
	FormatKind() Kind
	GetScore() shared.ScoreResult
}

// Format is the strategy interface every tournament format implements.
// Implementations live in their own file in this package and self-register
// from init() via register().
type Format interface {
	Name() Kind

	// Predictions
	RequiredPredictions(teamCount int) int
	GeneratePrediction(user shared.User, round string, teams []string) (shared.Prediction, error)

	// Scoring
	CalculateScore(p shared.Prediction, r MatchResult) (ScoreReport, error)

	// DB Interaction
	DecodeBSON(bytes []byte) (MatchResult, error)

	// Parsing
	BuildFromMatchNodes(nodes []external.MatchNode, round string) (MatchResult, error)
	ExtractMatchListIDs(wikitext string) ([]string, Kind, error)
}

// registry holds every Format known to the package, keyed by Kind.
// Populated at init time from each format's own file via register().
var registry = map[Kind]Format{}

// register adds f to the registry under its declared name. Panics on
// duplicate registration so collisions are caught at startup, not at runtime.
// Intended to be called only from init() in format implementation files.
func register(f Format) {
	name := f.Name()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("format: duplicate registration for %q", name))
	}
	registry[name] = f
}

// Get returns the Format registered under name, or an error if no format
// with that name is registered.
func Get(name Kind) (Format, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("format: unknown format %q", name)
	}
	return f, nil
}

// MustGet is like Get but panics on unknown name. Use only when the caller
// has already validated the name (e.g. in tests or trusted internal paths).
func MustGet(name Kind) Format {
	f, err := Get(name)
	if err != nil {
		panic(err)
	}
	return f
}

// Names returns the names of every registered Format. Order is not guaranteed.
func Names() []Kind {
	out := make([]Kind, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	return out
}

// DetectKind determines the format (Kind) of a tournament from the wikitext
func DetectKind(wikitext string) (Kind, error) {
	// Regex to find ==Format== section in wikitext
	re := regexp.MustCompile(`(?s)==\s*Format\s*==\s*(.*)`)
	results := re.FindStringSubmatch(wikitext)

	if len(results) > 1 {
		formatSection := results[1] // format is listed on the second line of the format section in wikitext
		switch {
		case strings.Contains(strings.ToLower(formatSection), "swiss") && strings.Contains(strings.ToLower(formatSection), "single-elimination"):
			// This case occurs when both styles are on a singular page. This doesnt occur during the major and is just here for testing
			return Swiss, nil
		case strings.Contains(strings.ToLower(formatSection), "swiss"):
			return Swiss, nil
		case strings.Contains(strings.ToLower(formatSection), "single-elimination"):
			return SingleElim, nil
		case strings.Contains(strings.ToLower(formatSection), "double-elimination"):
			return DoubleElim, nil
		default:
			return "", fmt.Errorf("==Format==' section did not match any registered format (Kind)")
		}
	}
	return "", fmt.Errorf("'==Format==' section found in wikitext")
}

func extractMatchListIds(wikitext string, re *regexp.Regexp) ([]string, Kind, error) {
	ids := []string{}
	format, err := DetectKind(wikitext)

	if err != nil {
		return nil, "", err
	}
	// Find regex matches
	matches := re.FindAllStringSubmatch(wikitext, -1)
	for _, match := range matches {
		paramsText := match[1]

		// Parse pipe ("|") seperated key value pairs from template
		parts := strings.Split(paramsText, "|")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "id=") {
				id := strings.TrimSpace(strings.TrimPrefix(part, "id="))

				// Remove trailing html comments (some times occurs in single elim data)
				reComment := regexp.MustCompile(`<!--.*?-->`)
				id = reComment.ReplaceAllString(id, "")
				id = strings.TrimSpace(id)

				if id != "" {
					ids = append(ids, id)
				}
				break // No need to parse more params
			}
		}
	}

	if len(ids) == 0 {
		return nil, "", fmt.Errorf("no ids found")
	}
	return ids, format, nil
}
