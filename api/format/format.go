// Package format provides the per-tournament-format strategy interface
// (Swiss, single-elimination, etc.) plus the registry that callers use to
// dispatch by format name. Concrete formats live in their own files in this
// package and self-register via init().
package format

import (
	"fmt"

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

// Format is the strategy interface every tournament format implements.
// Implementations live in their own file in this package and self-register
// from init() via register().
type Format interface {
	Name() Kind

	// Predictions
	RequiredPredictions(teamCount int) int
	GeneratePrediction(user shared.User, round string, teams []string) (shared.Prediction, error)

	// Scoring
	CalculateScore(p shared.Prediction, r MatchResult) (shared.ScoreResult, string, error)

	// DB Interaction
	DecodeBSON(bytes []byte) (MatchResult, error)
	BuildFromMatchNodes(nodes []external.MatchNode, round string) (MatchResult, error)

	// Parsing
	ParseFromAPI(jsonResponse, round string) (MatchResult, error)
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
