# Pick'Ems Bot
## About
This is a discord bot used to track Pick'Ems for the CS2 Copenhagen major. The source of data for this project is 
[Liquipedia](https://liquipedia.net/counterstrike/PGL/2024/Copenhagen). Originally, the bot did data scraping using soup to extract the data from the liquipedia match page,
however, this proved to be inefficient, unreliable, and cause unexpected errors. We now use the Liquipedia api and db to 
obtain information. This data is stored in our own database to reduce the number of calls we must make to Liquipedia's 
servers, reducing latency and complying with the [API usage requirements](https://liquipedia.net/api-terms-of-use).

## Bot Commands
The following are discord messages that the bot will respond to. These can be in a server the bot is added to or dm'd to the bot. Note that there is no server-specific rankings. It is all global \
`$set [team1] [team2] ... [team10]`: Sets your Pick'Ems. 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams, and 9-10 are the 0-3 teams. Please note that the teams names need to be specified exactly how they appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. \"The MongolZ\" \
`$check`: shows the current status of your Pick'Ems \
`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your Pick'Ems \
`$leaderboard`: shows which users have the best Pick'Ems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes \
`$upcoming`: shows todays live and upcoming matches

## Usage

### Configuration

The bot reads from two files:

- `.env` — secrets only (Discord tokens, Mongo URI, Liquipedia API key). Not tracked in git.
- `config.toml` — tournament settings (page, round, etc.). Tracked in git so the active tournament is committed alongside the code.

Required `.env` keys:

```
DISCORD_PROD_TOKEN=...
DISCORD_BETA_TOKEN=...
MONGO_PROD_URI=...
LIQUIDPEDIADB_API_KEY=...
```

### Generating config.toml for a new tournament

Use the configure script — point it at any Liquipedia tournament page and it'll fetch the wikitext, find the available stages, and write `config.toml`:

```bash
go run ./scripts/configure -url https://liquipedia.net/counterstrike/Intel_Extreme_Masters/2026/Cologne
```

If the tournament has multiple stages, you'll get an interactive picker. Skip the prompt with `-stage`:

```bash
go run ./scripts/configure -url https://liquipedia.net/counterstrike/PGL/2026/Bucharest -stage Europe
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