# Changelog

## 3.3
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
