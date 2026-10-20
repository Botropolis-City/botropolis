ALTER TABLE messages ADD COLUMN web_search_requests INTEGER;
ALTER TABLE messages ADD COLUMN web_fetch_requests INTEGER;
ALTER TABLE messages ADD COLUMN service_tier TEXT;
CREATE TABLE meta (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
INSERT INTO meta (key, value) VALUES ('readable_from', 1), ('written_by', 3);
PRAGMA user_version = 3;
