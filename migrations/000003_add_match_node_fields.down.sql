DROP INDEX IF EXISTS matches_tournament_external_id_idx;

ALTER TABLE matches
    DROP COLUMN IF EXISTS external_id,
    DROP COLUMN IF EXISTS score;