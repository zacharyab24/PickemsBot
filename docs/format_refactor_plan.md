# Match Format Refactor — Plan & Todo

Goal: collapse all per-format logic (Swiss, single-elimination, future double-elimination) behind a `Format` interface in `api/format/`, so adding a new format means writing one file and registering it — no other call sites change.

Lives at `api/format/`. Each format is one file (`swiss.go`, `single_elimination.go`, `double_elimination.go`). Callers dispatch via `format.Get(name)` instead of `switch format { case "swiss": ... }`.

---

## Design decisions (locked before Phase 0)

- **Prediction shape stays flat.** `store.Prediction` keeps its existing fields (`Win`, `Advance`, `Lose`, `Progression`). New formats add new fields rather than restructuring into `map[string][]string`. Avoids a breaking BSON migration. Revisit only if a format genuinely needs dynamic categories.
- **Score report (Discord string) stays inside `Format.CalculateScore`.** No data/presentation split — Discord is the only output target.
- **Wikitext parsing moves into format files** in Phase 3. `api/external/parser.go` becomes a transport/utility layer (HTTP, JSON envelope handling, format-agnostic helpers).
- **Format identity uses a named-string type `Kind`** with consts `Swiss` / `SingleElim` / `DoubleElim`. Keeps BSON values as plain strings (no migration). Type-checked at function boundaries. No `int` + `iota` (would break BSON round-trip).

---

## Phase status

- [x] **Phase 0** — Scaffold
  - [x] `api/format/format.go` — `Kind` type, consts, `Format` interface, registry (`register`/`Get`/`MustGet`/`Names`)
  - [x] `api/format/swiss.go` — stub: `Name()`, `RequiredPredictions()`, panics for the rest
  - [x] `api/format/single_elimination.go` — stub: `Name()`, `RequiredPredictions()`, panics for the rest
  - [ ] **Task #8 — `api/format/format_test.go`** ← next
- [ ] Phase 1 — Migrate scoring (Task #9)
- [ ] Phase 2 — Migrate storage round-trip (Task #10)
- [ ] Phase 3 — Migrate parsing (Task #11)
- [ ] Phase 4 — Migrate prediction generation + API layer (Task #12)
- [ ] Phase 5 — Sweep & lock down (Task #13)
- [ ] Phase 6 — Add DoubleElim to validate the abstraction (Task #14)

Each phase is a separate PR. Build green, tests green, coverage ≥80% at every step.

---

## Phase 0 — Scaffold (in progress)

**Goal:** new package exists, registry works, nothing calls it yet.

- [x] Create `api/format/format.go` with `Kind`, `Swiss`, `SingleElim` consts, `Format` interface, registry
- [x] Create `api/format/swiss.go` with stub impl
- [x] Create `api/format/single_elimination.go` with stub impl
- [ ] **Create `api/format/format_test.go`**
  - [ ] `TestGet_ReturnsSwiss` — `Get(Swiss)` returns the swiss impl, no error
  - [ ] `TestGet_ReturnsSingleElim` — same for single-elim
  - [ ] `TestGet_UnknownReturnsError` — `Get("nonsense")` returns an error
  - [ ] `TestMustGet_PanicsOnUnknown`
  - [ ] `TestNames_ContainsBothFormats`
  - [ ] `TestSwiss_RequiredPredictions` — table test: T=8→5, T=16→10, T=24→15, T=32→20
  - [ ] `TestSingleElim_RequiredPredictions` — table test: T=4→2, T=8→4, T=16→8, T=32→16
  - [ ] Don't test the panicking stubs — they get coverage in later phases

**Done when:** `go test ./api/format/...` passes and package coverage ≥80%.

---

## Phase 1 — Migrate scoring

**Goal:** `CalculateUserScore` becomes a one-liner; format-specific scoring lives with the format.

- [ ] Move `calculateSwissScore` (api/logic/input_processing.go:88) → `swissFormat.CalculateScore`
- [ ] Move `evaluateSwissPrediction` (api/logic/input_processing.go:153) → private helper inside `swiss.go`
- [ ] Move `calculateEliminationScore` (api/logic/input_processing.go:195) → `singleElimFormat.CalculateScore`
- [ ] Replace the type-switch in `CalculateUserScore` (api/logic/input_processing.go:66) with:
  ```go
  f, err := format.Get(format.Kind(results.GetType()))
  if err != nil { return store.ScoreResult{}, "", err }
  return f.CalculateScore(userPrediction, results)
  ```
- [ ] Move format-specific test cases from `input_processing_test.go` → `swiss_test.go` / `single_elimination_test.go`
- [ ] Keep one or two integration tests in `input_processing_test.go` that exercise the dispatch itself

**Done when:** the type-switch in `input_processing.go:66` is deleted; coverage stays ≥80%.

---

## Phase 2 — Migrate storage round-trip

**Goal:** record ↔ result conversion goes through the registry.

- [ ] Move `ToMatchResult` switch (api/store/models.go:120) into per-format `FromRecord` methods
- [ ] Implement `ToRecord` (inverse direction) on each format
- [ ] Collapse the three switches in `api/store/match_results.go` (lines 55, 123, 267) to registry dispatch
- [ ] Replace the type switch in `api/store/user_predictions.go:115` with `format.Get(rec.GetType()).FromRecord(rec)`
- [ ] Storage record types (`SwissResultRecord`, `EliminationResultRecord`) **stay** in `api/store/models.go` — they're storage types and `format` imports `store`, not the other way around

**Done when:** `match_results.go` has zero `case "swiss":` lines.

---

## Phase 3 — Migrate parsing

**Goal:** "what Swiss looks like" lives in `swiss.go`, not `parser.go`.

- [ ] Move format-specific parsing from `api/external/parser.go:418` (and surrounding) into per-format `ParseFromAPI` methods
- [ ] Move format-detection heuristic at `parser.go:471` into a top-level `format.Detect(wikitext string) Kind` in `api/format/format.go`
- [ ] Keep in `api/external`: `GetWikitext`, `GetLiquipediaMatchData`, `ExtractMatchListID` — these are HTTP/transport, format-agnostic

**Done when:** `api/external/parser.go` has zero `case "swiss":` switches.

---

## Phase 4 — Migrate prediction generation + API layer

**Goal:** API-layer switches go away.

- [ ] Move `GeneratePrediction` switch (api/logic/prediction.go:37) → per-format `GeneratePrediction` methods
- [ ] Move `setSwissPredictions` (api/logic/prediction.go:53) into `swissFormat.GeneratePrediction`
  - [ ] **While migrating, generalize the bucket slicing** to match the `RequiredPredictions` formula: `teams[0:T/8]`, `teams[T/8:T/2]`, `teams[T/2:5T/8]` instead of hardcoded `0:2` / `2:8` / `8:10`
- [ ] Move `setEliminationPredictions` (api/logic/prediction.go:64) into `singleElimFormat.GeneratePrediction`
- [ ] Replace switches in `api/api/api.go` lines 69 and 296 with registry dispatch

**Done when:** the only remaining switches on format string are inside the `format` package's own files.

---

## Phase 5 — Sweep & lock down

**Goal:** catch stragglers, harden the boundary.

- [ ] Run: `grep -rn '"swiss"\|"single-elimination"' --include="*.go"`
- [ ] Anything outside `api/format/` and BSON tags → convert to `format.Swiss` / `format.SingleElim` consts
- [ ] Update `api/store/test_helpers.go:75` to use the consts
- [ ] Update `api/api/test_mocks.go:185` to use the consts
- [ ] Consider adding a `var _ = []Format{swissFormat{}, singleElimFormat{}}` somewhere to anchor the registered types from accidental removal

**Done when:** zero string literals `"swiss"` or `"single-elimination"` exist outside `api/format/` and BSON tags.

---

## Phase 6 — Add DoubleElim (validates the abstraction)

**Goal:** prove the refactor worked. If this phase requires touching files outside `api/format/` and `store.Prediction`, the abstraction leaked — fix the leak before merging.

- [ ] Create `api/format/double_elimination.go` implementing `Format`
- [ ] Add `DoubleElim Kind = "double-elimination"` const in `format.go`
- [ ] Add fields to `store.Prediction` for upper/lower bracket categories
- [ ] Add `double_elimination_test.go`
- [ ] Verify the diff touches **only** `api/format/` + new `store.Prediction` fields

**Done when:** DoubleElim works end-to-end and Phase 6's diff is contained as described.

---

## Sanity rules for every phase

- **One PR per phase.** Don't combine.
- **Coverage stays ≥80%** (existing CI gate enforces this).
- `bash scripts/quality_check.sh` passes locally before pushing.
- Each PR opens with a one-liner: which switch / type-switch this PR deletes.
- Commit messages: conventional commits (`refactor(format): migrate scoring into format package`).

---

## File location reference

| What | Where |
|---|---|
| `Format` interface, `Kind`, registry | `api/format/format.go` |
| Swiss impl | `api/format/swiss.go` |
| Single-elim impl | `api/format/single_elimination.go` |
| Double-elim impl (Phase 6) | `api/format/double_elimination.go` |
| Storage record types | `api/store/models.go` (unchanged) |
| Result types (`SwissResult`, `EliminationResult`) | `api/external/models.go` (unchanged) |
| Prediction struct | `api/store/models.go` (new fields added in Phase 6 only) |
