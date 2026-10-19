// Package stats is Botropolis's statistics store: one row per assistant
// message — when, which model, which directory, which thread and its
// usage counters — kept in SQLite so the numbers outlive the transcripts
// they came from. It holds ids, times, model names, directory paths and
// counts, and never anything Claude or you wrote. Item 110.
package stats

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"botropolis.city/botropolis/pkg/statedir"
)

const (
	file = "stats.db"
	// Version is the schema this build writes, kept in PRAGMA user_version.
	Version = 2
)

const schema = `
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
  cache_read   INTEGER NOT NULL,
  cache_create_5m INTEGER,
  cache_create_1h INTEGER,
  geo             TEXT
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
`

// steps brings a database from the version before each key to that
// version. A new database is made at Version by schema directly.
var steps = map[int]string{
	// 2 keeps cache writes split by lifetime, and the inference
	// geography, which are priced differently (item 116). Existing
	// messages keep an unknown split, and every transcript is read
	// again from the start, so the next ingest fills in whatever is
	// still on disk.
	2: `ALTER TABLE messages ADD COLUMN cache_create_5m INTEGER;
ALTER TABLE messages ADD COLUMN cache_create_1h INTEGER;
ALTER TABLE messages ADD COLUMN geo TEXT;
UPDATE transcripts SET read_to = 0;`,
}

// Store is an open statistics database.
type Store struct {
	db *sql.DB
}

// Path is where the statistics database lives: in the state directory,
// so a private run gets a private database (item 111).
func Path() string {
	return filepath.Join(statedir.Dir(), file)
}

// Open opens the database at path, creating it readable by its owner
// only, and brings an older schema up to Version. A database written by
// a newer build is refused rather than read wrongly.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(path string) error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > Version {
		return fmt.Errorf("%s is schema version %d, newer than this build's %d: update Botropolis", path, version, Version)
	}
	if version == Version {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if version == 0 {
		if _, err := tx.Exec(schema); err != nil {
			return err
		}
	} else {
		for v := version + 1; v <= Version; v++ {
			if _, err := tx.Exec(steps[v]); err != nil {
				return fmt.Errorf("%s: bringing the schema to version %d: %w", path, v, err)
			}
		}
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", Version)); err != nil {
		return err
	}
	return tx.Commit()
}

// DB is the database itself, for the packages that read and write it.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }
