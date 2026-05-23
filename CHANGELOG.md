# Changelog

## 3.3
PandaScore integration — second data source alongside Liquipedia:
- Added PandaScore as a supported `data_source` in `config.toml`. Set `data_source = "pandascore"` and `series_id = <id>` to use it; `data_source = "liquipedia"` retains the existing webhook-driven behaviour.
- `store.DataSourceFetcher` interface (`FetchMatchData`, `FetchSchedule`) with `LiquipediaFetcher` and `PandaScoreFetcher` implementations — makes it straightforward to add more sources in the future.
- `web.Poller` — polls the PandaScore API once per minute and triggers `UpdateMatchResults` + `GenerateLeaderboard` + `RenderResultsImage` on the first tick where any match transitions to `finished`. Stops automatically on unrecoverable errors (401/403/404) via `sources.ErrUnrecoverable`.
- `MatchNode.Status` field added (transient, not persisted) so the poller can track per-match status transitions without an extra DB round-trip.
- `sources/pandascore.go`: fixed API endpoint (`/csgo/matches?filter[serie_id]=…` instead of the non-existent `/csgo/series/{id}/matches`); Twitch stream priority over Kick for `$upcoming` match links.
- `sources/liquipedia.go`: switched to `streamurls=true` to get full stream URLs directly from the API; stream parsing now falls back gracefully (empty URL) instead of erroring when no Twitch/Kick key is present.
- `scripts/configure`: added `-source` flag (`liquipedia` | `pandascore`); PandaScore path accepts `-series-id`, `-name`, `-round` flags and writes a PandaScore-specific `config.toml` (no `page`/`format` fields).
- `scripts/fetchtest`: new smoke-test tool — `go run ./scripts/fetchtest <liquipedia|pandascore> <page|seriesID> <round>` — verifies both data sources return well-formed match data before deploying.
- `Dockerfile`: added comments listing all required runtime environment variables (`PANDASCORE_API_KEY` added alongside the existing secrets).

Structured logging uplift — `log` → `slog` with JSON output:
- Replaced all `log.*` calls in the bot runtime (`main`, `app`, `bot`, `store`, `web`) with `log/slog`. Logs are now emitted as structured JSON to stdout, ready for Promtail → Loki ingestion without any code changes.
- Log level is configurable via `log_level` in `config.toml` (`debug`, `info`, `warn`, `error`). Defaults to `debug` when `test = true`, `info` otherwise.
- Each major component (`app`, `store`, `bot`, `poller`, `web`) has its own injected `*slog.Logger` tagged with a `component` field, so every log line identifies its source without relying on message text.
- `log.Fatalf` calls in Discord handlers replaced with `slog.Error` + `return` — a failed embed send no longer crashes the entire bot process.
- Errors at log sites wrapped with `fmt.Errorf("functionName: %w", err)` to provide call-site breadcrumbs in the absence of stack traces.
- Duplicate log entries eliminated: rate-limit events were previously logged in `app.go` and again by the caller; now logged once at the handling site only.
- Poller continuable errors (transient fetch failures, parse errors, update/render failures) are `Warn`; only an unrecoverable API error that stops the poller is `Error`.
- `scripts/configure` and `scripts/fetchtest` left unchanged — their `fmt.Printf` output is intentional CLI output, not application logging.

Bug fixes and data layer improvements:
- Fixed Swiss `$check` off-by-one: 3-0 and 0-3 buckets each showed 1 team instead of 2, advance showed 4 instead of 6. Root cause: `setSwissPredictions` divided the input list length by the wrong denominator.
- Fixed `$results` for single-elimination: trimmed the 3rd-place consolation match that Liquipedia's `Bracket/8` template appends as an 8th node (the renderer only supports the 7-match main bracket). Also fixed column layout — Liquipedia returns all bracket nodes with `Section = "Bracket/8"`, causing the renderer to stack all matches in one column; sections are now normalised to `Quarterfinals` / `Semifinals` / `Grand Final` by match position before rendering.
- Bot no longer crashes on startup if a stored prediction can't be scored against the current results (e.g. stale entry from a previous format or round); it logs a warning and skips it.
- Switched LiquipediaDB data fetching to a `[[pagename::X]]` query — bracket IDs no longer need to be scraped from wikitext at runtime.
- `scripts/configure`: added `-format` flag to write the `format` field in `config.toml`; required for multi-stage tournament pages (e.g. `-format swiss` for a group stage).
- Moved wikitext HTTP fetch logic from `api/external` into the configure script package — `api/external` now only contains bot runtime calls.

Discord embed uplift — all bot responses now use rich embeds instead of plain text strings:
- `$check` — format-aware embed; Swiss shows three prediction buckets (3-0 / Advance / 0-3), single-elimination shows a sorted predictions list with trophy/medal position emojis (🏆 Champion, 🥈 Runner-up, 🥉 3rd/4th, 🎖️ Top 8) and status emojis (✅ / ⏳ / ❌).
- `$set` — green success embed on save; red error embed (with the error message) on failure.
- `$teams` — green embed with a two-column team layout and a footer showing the total team count.
- `$upcoming` — green embed with one field per match; each field shows a Discord timestamp (absolute + relative) and an optional 📺 watch-live link.
- All error responses across every handler converted from plain strings to red embeds via a shared `sendError` helper.
- `$results` intentionally left as a plain image attachment — no embed wrapper needed.
- `GetUpcomingMatches` refactored to return `[]external.ScheduledMatch` instead of pre-formatted strings, moving all presentation logic into the handler. Stream URLs are resolved to full Twitch links before being returned.
- `CalculateScore` / `CalculateUserScore` / `CheckPrediction` pipeline refactored to return structured `ScoreReport` types (`SwissReport`, `SingleElimReport`) instead of raw strings, enabling format-specific embed layouts.
- Updated all tests broken by the above return-type changes.

## 3.2
Match format rework and `$results` command:
- Reworked the match result layer to support multiple tournament formats (Swiss, single-elimination) through a shared `MatchResult` interface and format-specific implementations. Swiss and single-elimination formats now each have their own result type, scoring logic, and BSON encoding, making it straightforward to add new formats (e.g. double-elimination) in the future.
- Extended `MatchNode` with `Score` (series score e.g. `2-1` for BoX, map score e.g. `13-10` for BO1) and `Section` (round label e.g. `Round 1`, `Upper Bracket Round 2`) to carry enough information to reconstruct a full bracket across all supported formats.
- Persists raw match node data to a new `match_nodes` MongoDB collection after each update, decoupling bracket display from match result scoring.
- Added `$results` command — sends the current bracket as a rendered image in Discord.
- Integrated [`pickems-renderer`](https://github.com/zacharyab24/pickems-renderer) for bracket image generation; the renderer is triggered asynchronously after each Liquipedia webhook update.
- Migrated the storage layer to a `store.Interface` to allow mock injection in tests.
- Dockerfile reworked to a multi-stage build (Go 1.26 builder + `debian:bookworm-slim` runtime) with Chromium installed to support the bracket renderer.

Also includes changes from PRs #32 and #33:
- (#32) Added `scripts/configure` — a CLI tool that reads a Liquipedia tournament page and generates `config.toml`. Tournament settings are now tracked in git separately from secrets, which remain in `.env`.
- (#33) Fixed CI workflow vulnerability check stage and updated govulncheck integration.

## 3.1
Bot now spins up a web server that receives callbacks from Liquipedia when a page is updated. This means we automatically
update our cached data, instead of relying on users making commands. This means responses are faster when previously the
cache would be expired. Some other minor improvements and changes have been made and test coverage has been added.

## 3.0
- Reworked the application into two parts: `api` and `bot`
    - `api` is a restful api that is used for data retrieval. Lookups will be to the Liquipedia Database instead of scraping
    the site using soup. Most of the data will be stored in our own database, as to not exceed the usage requirements of the
    liquipedia
    - `bot` will make api calls to GET and POST data, instead of doing its own database interaction and web scraping. This
    will allow for a smoother experience, cleaner code, and less errors.
- fuzzy string matching: exact string matches are no longer required for entering team names during predictions. Example usage:
    - `fq` -> `FlyQuest`
    - `mongols` -> `"The MongolZ"`

## 2.0
Updated the code base to use Go instead of Python.
Updated to work with upcoming Perfect World Shanghai Major as well as be more expandable for other tournaments through the use of command line flags (not user facing).

## 1.1
Added upcoming match support. This may still be broken. I have to wait for today's matches to be finished to check. \
Updated help command. \
Updated formatting for check command.

## 1.0
Launched bot. \
Added allowing any capitalisation of teams. \
Added error handling for incorrect inputs.
