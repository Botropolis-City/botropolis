ALTER TABLE messages ADD COLUMN cache_create_5m INTEGER;
ALTER TABLE messages ADD COLUMN cache_create_1h INTEGER;
ALTER TABLE messages ADD COLUMN geo TEXT;
PRAGMA user_version = 2;
