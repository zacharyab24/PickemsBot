# Bracket Image Renderer — Project Brief

## What this project is

A standalone Go module that renders tournament bracket/results data to a PNG image. It will be imported as a dependency by the main PickemsBot project (not run as a microservice). The bot calls it in-process.

---

## Integration contract

### Input

The renderer receives match data already fetched from MongoDB via `FetchMatchNodesFromDb()`. The calling side provides a slice of `MatchNode` and the format kind:

```go
type MatchNode struct {
    ID      string // Liquipedia match2id, e.g. "IykJinz1G8_0001"
    Team1   string
    Team2   string
    Winner  string // team name, or "TBD" if unplayed
    Score   string // "2-1" (series) or "13-10" (BO1 map score); "" if unplayed
    Section string // round label, e.g. "Round 1", "Upper Bracket Round 2"
}
```

The document also carries a `format` field (`"swiss"`, `"single-elimination"`, `"double-elimination"`) so the renderer knows how to lay out the bracket.

### Output

A PNG written to a caller-supplied file path. The public API should be something like:

```go
func RenderBracket(nodes []MatchNode, format string, outputPath string) error
```

### Grouping

Nodes should be grouped by `Section` to produce rounds. Section values are string labels from Liquipedia:

- **Swiss:** `"Round 1"` through `"Round 5"` — sort by parsing the integer suffix
- **Single-elim:** Liquipedia naming convention e.g. `"Quarterfinals"`, `"Semifinals"`, `"Grand Final"` — sort by known fixed bracket order
- **Double-elim:** Expected to follow `"Upper Bracket Round 1"`, `"Lower Bracket Round 1"`, etc. — not yet confirmed against live data

---

## POC — already validated

A working POC lives at `go-imagegen-poc/` in the PickemsBot repo. It proves out the full HTML → PNG pipeline. The renderer project should start from this POC's approach.

### POC structure

```
go-imagegen-poc/
├── go.mod
├── main.go
└── render/
    ├── types.go    — Match, Round, Bracket structs
    ├── html.go     — HTML/CSS template + buildHTML()
    ├── chrome.go   — chromedp screenshot logic
    └── render.go   — public RenderBracket(Bracket, outputPath) entrypoint
```

The POC's `Bracket` struct (`Rounds → Matches → Team1/Team2/Score/Winner`) maps directly to what you get by grouping `[]MatchNode` by `Section`.

### Key implementation constraints

- **Chrome binary path:** Must be set explicitly on Fedora: `chromedp.ExecPath("/usr/bin/chromium-browser")` — the default searches for `google-chrome` which does not exist on Fedora
- **Docker:** `--no-sandbox` flag required inside Docker
- **HTML loading:** Write HTML to a temp file and load via `file://` URL — more reliable than `chromedp.SetContent` for complex CSS
- **Connector lines:** Bracket connector lines use CSS border technique (`border-right` + `border-top/bottom` forming a `]` shape)
- **Render cost:** 500ms–2s per render — must **not** be called synchronously on a Discord command

---

## Caching

- PNG is cached to the filesystem keyed by round
- Invalidated via an `imageStale` flag on the round document, set by the bot when new match results are stored
- Regeneration runs in the background when results update, not on-demand per command
- At command time: if stale, trigger background regeneration and respond with the cached (previous) image or a "rendering" message

---

## Go module setup

- Create a new repo (e.g. `github.com/zacharyab24/pickems-renderer`) with its own `go.mod`
- Module name must be importable: `module github.com/zacharyab24/pickems-renderer`
- Expose a clean public API in the root or a single `render` package
- During development before the module is published, use a `replace` directive in PickemsBot's `go.mod`:

```
replace github.com/zacharyab24/pickems-renderer => ../pickems-renderer
```

- The `MatchNode` type will either be re-declared in the renderer package (simple structs, no logic) or the renderer accepts a generic interface — coordinate with PickemsBot to avoid circular dependencies

---

## What the bot side will do

When implementing the `$results` command in PickemsBot:

1. Call `store.FetchMatchNodesFromDb()` to get `[]external.MatchNode`
2. Check `imageStale` flag
3. If stale or no cached image: call renderer in a goroutine, respond with text results in the meantime
4. Send cached PNG as a Discord attachment for explicit bracket display requests

### Discord delivery

- **Embeds** — text-based results summary, sent immediately
- **PNG attachment** — for explicit `$bracket` or similar requests, sent alongside an embed
- Both coexist, serving different use cases
