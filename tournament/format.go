// Package tournament provides the per-tournament-format strategy interface
// (Swiss, single-elimination, etc.) plus the registry that callers use to
// dispatch by format name. Concrete formats live in their own files in this
// package and self-register via init().
package tournament

import (
	"fmt"
	"strings"

	"pickems-bot/models"
	"pickems-bot/sources"
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
	GetScore() models.ScoreResult
}

// Format is the strategy interface every tournament format implements.
// Implementations live in their own file in this package and self-register
// from init() via register().
type Format interface {
	Name() Kind

	// Predictions
	RequiredPredictions(teamCount int) int
	GeneratePrediction(user models.User, round string, teams []string) (models.Prediction, error)

	// Scoring
	CalculateScore(p models.Prediction, r MatchResult) (ScoreReport, error)

	// PredictionFields returns a format-specific summary of p suitable for
	// display (e.g. Discord embed fields). Returns an error if the prediction
	// is malformed for this format (e.g. wrong field set populated).
	PredictionFields(p models.Prediction) ([]models.PredictionField, error)

	// DB Interaction
	DecodeBSON(bytes []byte) (MatchResult, error)

	// Parsing
	BuildFromMatchNodes(nodes []sources.MatchNode, round string) (MatchResult, error)
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

// FilterNodesByKind keeps only the match nodes whose Section field is relevant
// to the given format. This is necessary when a Liquipedia page contains mixed
// content — e.g. Swiss rounds alongside a playoffs bracket and showmatches.
// Filtering before scoring prevents cross-format matches from corrupting results.
//
// Rules:
//   - Swiss: keep nodes whose section contains "round"
//   - SingleElim: keep nodes whose section contains bracket/final/playoff keywords,
//     stripping Swiss rounds ("Round N") and showmatches
//   - DoubleElim: keep all nodes (always on their own page in practice)
func FilterNodesByKind(nodes []sources.MatchNode, kind Kind) []sources.MatchNode {
	switch kind {
	case Swiss:
		filtered := nodes[:0:0]
		for _, n := range nodes {
			if strings.Contains(strings.ToLower(n.Section), "round") {
				filtered = append(filtered, n)
			}
		}
		return filtered
	case SingleElim:
		// Keep bracket/final sections; discard Swiss rounds and showmatches.
		// Covers both naming conventions used on Liquipedia:
		//   "Bracket/8", "Quarterfinals", "Semifinals", "Grand Final", "Playoffs"
		filtered := nodes[:0:0]
		for _, n := range nodes {
			s := strings.ToLower(n.Section)
			if strings.Contains(s, "bracket") ||
				strings.Contains(s, "final") ||
				strings.Contains(s, "playoff") {
				filtered = append(filtered, n)
			}
		}
		return filtered
	default:
		return nodes
	}
}

// DetectKindFromMatchNodes infers the tournament format from the Section fields
// present in a slice of match nodes returned by the LiquipediaDB API.
// Priority: DoubleElim (upper + lower keywords) > Swiss (round keyword) > SingleElim (final keywords).
// Returns an error if no section keywords match any known format.
func DetectKindFromMatchNodes(nodes []sources.MatchNode) (Kind, error) {
	var hasRound, hasFinal, hasUpper, hasLower bool
	for _, n := range nodes {
		s := strings.ToLower(n.Section)
		if strings.Contains(s, "round") {
			hasRound = true
		}
		if strings.Contains(s, "final") || strings.Contains(s, "quarterfinal") || strings.Contains(s, "semifinal") {
			hasFinal = true
		}
		if strings.Contains(s, "upper") {
			hasUpper = true
		}
		if strings.Contains(s, "lower") {
			hasLower = true
		}
	}
	switch {
	case hasUpper && hasLower:
		return DoubleElim, nil
	case hasRound:
		return Swiss, nil
	case hasFinal:
		return SingleElim, nil
	default:
		return "", fmt.Errorf("could not detect tournament format from match node sections")
	}
}
