-- Core identity
CREATE TABLE guilds (
    guild_id TEXT PRIMARY KEY
);

CREATE TABLE users (
    user_id  TEXT PRIMARY KEY,
    username TEXT NOT NULL
);

-- Teams and rankings
CREATE TABLE teams (
    id             SERIAL PRIMARY KEY,
    canonical_name TEXT NOT NULL,
    source         TEXT NOT NULL,
    external_id    TEXT NOT NULL,
    UNIQUE (source, external_id)
);

CREATE TABLE team_rankings (
    team_id        INTEGER     NOT NULL REFERENCES teams(id),
    standing       INTEGER     NOT NULL,
    points         INTEGER     NOT NULL,
    roster         TEXT[]      NOT NULL,
    standings_date DATE        NOT NULL,
    synced_at      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (team_id, standings_date)
);

-- Tournaments and matches
CREATE TABLE tournaments (
    id          SERIAL PRIMARY KEY,
    external_id TEXT NOT NULL,
    source      TEXT NOT NULL,
    name        TEXT NOT NULL,
    format      TEXT NOT NULL,
    series_id   TEXT,
    UNIQUE (source, external_id)
);

CREATE TABLE matches (
    id            SERIAL  PRIMARY KEY,
    tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
    round         TEXT    NOT NULL,
    team1_id      INTEGER REFERENCES teams(id),
    team2_id      INTEGER REFERENCES teams(id),
    winner_id     INTEGER REFERENCES teams(id),
    scheduled_at  TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'pending'
);

CREATE TABLE match_maps (
    id          SERIAL  PRIMARY KEY,
    match_id    INTEGER NOT NULL REFERENCES matches(id),
    map_name    TEXT,
    team1_score INTEGER,
    team2_score INTEGER,
    winner_id   INTEGER REFERENCES teams(id)
);

-- Guild configuration
CREATE TABLE guild_config (
    id                      SERIAL  PRIMARY KEY,
    guild_id                TEXT    NOT NULL REFERENCES guilds(guild_id),
    tournament_id           INTEGER REFERENCES tournaments(id),
    round                   TEXT,
    results_channel_id      TEXT,
    notification_channel_id TEXT,
    UNIQUE (guild_id, tournament_id),
    UNIQUE (guild_id, results_channel_id)
);

-- Predictions and picks
CREATE TABLE predictions (
    id            SERIAL  PRIMARY KEY,
    user_id       TEXT    NOT NULL REFERENCES users(user_id),
    guild_id      TEXT    NOT NULL REFERENCES guilds(guild_id),
    tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
    round         TEXT    NOT NULL,
    format        TEXT    NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, guild_id, tournament_id, round)
);

CREATE TABLE swiss_picks (
    prediction_id INTEGER NOT NULL REFERENCES predictions(id),
    team_id       INTEGER NOT NULL REFERENCES teams(id),
    bucket        TEXT    NOT NULL CHECK (bucket IN ('win', 'advance', 'lose')),
    PRIMARY KEY (prediction_id, team_id)
);

CREATE TABLE single_elim_picks (
    prediction_id    INTEGER NOT NULL REFERENCES predictions(id),
    team_id          INTEGER NOT NULL REFERENCES teams(id),
    predicted_round  TEXT    NOT NULL,
    predicted_status TEXT    NOT NULL CHECK (predicted_status IN ('advanced', 'eliminated')),
    PRIMARY KEY (prediction_id, team_id)
);

-- Materialised scores, updated on match result insert
CREATE TABLE scores (
    prediction_id    INTEGER PRIMARY KEY REFERENCES predictions(id),
    successes        INTEGER     NOT NULL DEFAULT 0,
    pending          INTEGER     NOT NULL DEFAULT 0,
    failed           INTEGER     NOT NULL DEFAULT 0,
    last_computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
