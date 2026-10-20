CREATE TABLE IF NOT EXISTS dirs (
  id   INTEGER PRIMARY KEY,
  path TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS sessions (
  id                 TEXT PRIMARY KEY,
  started_at         INTEGER,
  transcript_gone_at INTEGER
);

CREATE TABLE IF NOT EXISTS messages (
  id           TEXT PRIMARY KEY,
  at           INTEGER NOT NULL,
  model        TEXT NOT NULL,
  speed        TEXT,
  dir_id       INTEGER NOT NULL REFERENCES dirs,
  thread       TEXT NOT NULL,
  input        INTEGER NOT NULL,
  output       INTEGER NOT NULL,
  cache_create INTEGER NOT NULL,
  cache_read   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS message_sessions (
  message_id TEXT NOT NULL REFERENCES messages,
  session_id TEXT NOT NULL REFERENCES sessions,
  PRIMARY KEY (message_id, session_id)
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS message_sessions_by_session ON message_sessions (session_id, message_id);
CREATE INDEX IF NOT EXISTS messages_by_at     ON messages (at);
CREATE INDEX IF NOT EXISTS messages_by_dir_at ON messages (dir_id, at);

CREATE TABLE IF NOT EXISTS transcripts (
  path       TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions,
  size       INTEGER NOT NULL,
  mtime      INTEGER NOT NULL,
  read_to    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS gaps (
  off_at INTEGER NOT NULL,
  on_at  INTEGER NOT NULL
);
