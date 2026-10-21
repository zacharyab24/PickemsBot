ALTER TABLE matches
    ADD COLUMN team1_name TEXT,
    ADD COLUMN team2_name TEXT,
    ADD COLUMN best_of    TEXT,
    ADD COLUMN stream_url TEXT,
    ADD COLUMN is_live    BOOLEAN NOT NULL DEFAULT FALSE;