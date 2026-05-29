# Pick'Ems Bot
## About
This is a discord bot used to track Pick'Ems for the CS2 Copenhagen major. The source of data for this project is 
[Liquipedia](https://liquipedia.net/counterstrike). Originally, the bot did data scraping using soup to extract the data from the liquipedia match page, however, this proved to be inefficient, unreliable, and cause unexpected errors. We now use the Liquipedia api and db to obtain information. This data is stored in our own database to reduce the number of calls we must make to Liquipedia's servers, reducing latency and complying with the [API usage requirements](https://liquipedia.net/api-terms-of-use).

## Bot Commands
The following are discord messages that the bot will respond to. These can be in a server the bot is added to or dm'd to the bot. Note that there is no server-specific rankings. It is all global
- `$set [team1] [team2] ... [team10]`: Sets your Pick'Ems. 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams, and 9-10 are the 0-3 teams. Please note that the teams names need to be specified exactly how they appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. `"The MongolZ"`. Note, the bot now supports fuzzy matching, so `Mongolz` \(or probably even just `mongols`) would work too.
- `$check`: shows the current status of your Pick'Ems
- `$teams`: shows the teams currently in the current stage of the tournament, sorted by VRS world ranking. Use this list to set your Pick'Ems.
- `$team <name>`: looks up a team's current VRS world ranking, points total, and roster. Fuzzy matching applies, so approximate names work.
- `$leaderboard`: shows which users have the best Pick'Ems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes
- `$upcoming`: shows todays live and upcoming matches
- `$results`: shows the match results for the current round of the tournament including: team names, bracket position, match score. This is handled by a seperate module, which can be found [here](https://github.com/zacharyab24/pickems-renderer)

## Usage

### Configuration

The bot reads from two files:

- `.env` — secrets only (Discord tokens, Mongo URI, Liquipedia API key). Not tracked in git.
- `config.toml` — tournament settings (page, round, etc.). Not tracked in git; generate it with `go run ./scripts/configure` (see below).

Required `.env` keys:

```
DISCORD_PROD_TOKEN=...
DISCORD_BETA_TOKEN=... (optional but useful for testing)
MONGO_PROD_URI=...
LIQUIDPEDIADB_API_KEY=...
```
Note: this repo does not contain these secrets. If you wish to self host this bot, you are responsible for obtaining your own liquipedia key. Checkout the [liquipedia api](https://liquipedia.net/api) for more information

### Generating config.toml for a new tournament

Use the configure script — point it at any Liquipedia tournament page and it'll fetch the wikitext, find the available stages, and write `config.toml`:

```bash
go run ./scripts/configure -url https://liquipedia.net/counterstrike/Intel_Extreme_Masters/2026/Cologne
```

If the tournament has multiple stages, you'll get an interactive picker. Skip the prompt with `-stage`:

```bash
go run ./scripts/configure -url https://liquipedia.net/counterstrike/PGL/2026/Bucharest -stage Europe
```

For multi-stage pages where auto-detection would pick the wrong format, pass `-format`:

```bash
go run ./scripts/configure -url https://liquipedia.net/counterstrike/Intel_Extreme_Masters/2026/Cologne -stage Stage_1 -format swiss
```

### Running

```bash
go run .
```

Or via Docker (persistent — survives shell exit):

```bash
docker build -t pickems-bot .
docker run --env-file .env -v "$PWD/config.toml:/app/config.toml:ro" pickems-bot
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history.
