# Pick'Ems Bot
## About
A Discord bot for running CS2 Pick'Ems in your server. Members predict how a tournament stage will play out, and the bot scores those predictions as results come in.

Match data comes from [PandaScore](https://pandascore.co) (polled) or [Liquipedia](https://liquipedia.net/counterstrike) (webhook driven), and VRS world rankings from Valve's [regional standings](https://github.com/ValveSoftware/counter-strike_regional_standings). Everything is stored in PostgreSQL so the bot makes as few calls to the data sources as possible, in line with their [API terms](https://liquipedia.net/api-terms-of-use).

## Commands
Each channel follows its own tournament and round, chosen with `/config`. Predictions and leaderboards are per server.

- `/config view | set-tournament | set-round`: choose which tournament and round this channel tracks. Requires the Manage Server permission. Nothing else works in a channel until it is configured.
- `/set`: set your Pick'Ems from a form of team dropdowns. Swiss stages ask for two 3-0 teams, six advancing teams and two 0-3 teams; single elimination asks for the winner, runner-up and both 3rd/4th place teams.
- `/check [user]`: show your Pick'Ems, or another user's, and how they are scoring.
- `/leaderboard`: rank everyone in this server by correct picks. There is no tiebreaker.
- `/upcoming`: show live and upcoming matches.
- `/results`: show match results, grouped by round and paged for large stages.
- `/teams`: list the teams in the current stage, sorted by VRS world ranking.
- `/team <name>`: show a team's VRS ranking, points and roster. Team names autocomplete.

Predictions are supported for Swiss and single elimination stages. Other formats (double elimination, round robin) still get `/upcoming` and `/results`.

## Self-hosting

### Configuration

The bot reads from two files, neither of which is tracked in git:

- `.env`: secrets.
- `config.toml`: deployment settings. Generate it with `go run ./scripts/configure` (see below).

`.env` keys:

```
DISCORD_PROD_TOKEN=...
DISCORD_BETA_TOKEN=...      # used when config.toml has test = true
POSTGRES_URI=...
PANDASCORE_API_KEY=...      # PandaScore data, and the tournament list /config picks from
LIQUIDPEDIADB_API_KEY=...   # only needed for Liquipedia tournaments
```

You are responsible for obtaining your own API keys. See the [PandaScore](https://developers.pandascore.co) and [Liquipedia](https://liquipedia.net/api) API docs.

### Database

The schema lives in `migrations/` and is managed with [golang-migrate](https://github.com/golang-migrate/migrate). The bot does not apply migrations itself; run them before starting it:

```bash
migrate -path migrations -database "$POSTGRES_URI" up
```

### Generating config.toml

For PandaScore, tournaments are picked per channel with `/config`, so the config only needs the data source:

```bash
go run ./scripts/configure -source pandascore
```

For Liquipedia, point the script at a tournament page. It fetches the page, finds the available stages and writes the page and round the webhook server should update:

```bash
go run ./scripts/configure -source liquipedia -url https://liquipedia.net/counterstrike/Intel_Extreme_Masters/2026/Cologne
```

If the tournament has multiple stages you'll get an interactive picker. Skip it with `-stage`:

```bash
go run ./scripts/configure -source liquipedia -url https://liquipedia.net/counterstrike/PGL/2026/Bucharest -stage Europe
```

Optional settings you can add to `config.toml` by hand:

- `test = true`: use the beta Discord token and debug logging.
- `log_level = "debug" | "info" | "warn" | "error"`
- `dev_guild_id = "..."`: register slash commands in one server only, which updates instantly while testing.

### Running

```bash
go run .
```

Or via Docker:

```bash
docker build -t pickems-bot .
docker run --env-file .env -v "$PWD/config.toml:/app/config.toml:ro" -p 8080:8080 -p 9090:9090 pickems-bot
```

`.env` and `config.toml` are excluded from the image (see `.dockerignore`), so they must be passed in at runtime as above. Port 8080 serves the Liquipedia webhook; port 9090 serves `/health`, `/poller` and Prometheus metrics.

## Development

`./scripts/quality_check.sh` runs the same checks as CI: formatting, build, vet, lint, staticcheck, tests with an 80% coverage gate, and govulncheck. Store tests run against a throwaway Postgres container, so Docker must be running. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history.
