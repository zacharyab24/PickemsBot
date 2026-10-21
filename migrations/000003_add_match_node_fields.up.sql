ALTER TABLE matches
    ADD COLUMN external_id TEXT,
    ADD COLUMN score       TEXT;

CREATE UNIQUE INDEX matches_tournament_external_id_idx ON matches (tournament_id, external_id)
    WHERE external_id IS NOT NULL;
