UPDATE tournaments SET format = '' WHERE format IS NULL;
ALTER TABLE tournaments ALTER COLUMN format SET NOT NULL;
ALTER TABLE tournaments ALTER COLUMN format DROP DEFAULT;
