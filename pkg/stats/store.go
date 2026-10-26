// Package stats is Botropolis's statistics store: one row per assistant
// message — when, which model, which directory, which thread and its
// usage counters — kept in SQLite so the numbers outlive the transcripts
// they came from. It holds ids, times, model names, directory paths and
// counts, and never anything Claude or you wrote. Item 110.
package stats

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	_ "modernc.org/sqlite"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/statedir"
)

const (
	file = "stats.db"
	// Version is the schema this build writes, kept in PRAGMA user_version.
	Version = 3
	// ReadableFrom is the oldest schema version whose builds can still
	// read and write a database at Version correctly, kept in meta. A
	// step that only adds nullable columns leaves it alone; a step that
	// changes what a column means raises it to Version.
	ReadableFrom = 1
	// CacheKiB is each connection's page cache. SQLite's default, 2,000
	// KiB, cost the daemon about 4 MiB at 2,000 sessions, and a quarter
	// of it ingests as fast (item 211).
	CacheKiB = 512
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
  geo             TEXT,
  web_search_requests INTEGER,
  web_fetch_requests  INTEGER,
  service_tier        TEXT
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

CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value INTEGER NOT NULL
);
`

// movedSchema records, in the database, every session whose counters a
// write changed, one row each, so a reader can read again only those —
// whichever process or build wrote. It changes no table an older build
// knows, so it is made on every open rather than by a version step.
// Item 196.
const movedSchema = `
CREATE TABLE IF NOT EXISTS moved (
  session_id TEXT PRIMARY KEY
) WITHOUT ROWID;

CREATE TRIGGER IF NOT EXISTS moved_on_link AFTER INSERT ON message_sessions BEGIN
  INSERT INTO moved (session_id) VALUES (NEW.session_id) ON CONFLICT DO NOTHING;
END;

CREATE TRIGGER IF NOT EXISTS moved_on_unlink AFTER DELETE ON message_sessions BEGIN
  INSERT INTO moved (session_id) VALUES (OLD.session_id) ON CONFLICT DO NOTHING;
END;

CREATE TRIGGER IF NOT EXISTS moved_on_usage AFTER UPDATE OF input, output, cache_create, cache_read ON messages
WHEN OLD.input IS NOT NEW.input OR OLD.output IS NOT NEW.output
  OR OLD.cache_create IS NOT NEW.cache_create OR OLD.cache_read IS NOT NEW.cache_read
BEGIN
  INSERT INTO moved (session_id) SELECT session_id FROM message_sessions WHERE message_id = NEW.id ON CONFLICT DO NOTHING;
END;
`

// changeSchema logs, in the database, every message and every session a
// write changed, one row each under the latest seq from one counter, so
// a reader in another process can bring what it read up to date from
// the rows past the seq it last read. One row each bounds it whatever
// writes, an older build that never reads it included. Like movedSchema
// it changes no table an older build knows, so it is made on every
// open. Item 215.
const changeSchema = `
CREATE TABLE IF NOT EXISTS changed_messages (
  message_id TEXT PRIMARY KEY,
  seq        INTEGER NOT NULL
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS changed_messages_by_seq ON changed_messages (seq);

CREATE TABLE IF NOT EXISTS changed_sessions (
  session_id TEXT PRIMARY KEY,
  seq        INTEGER NOT NULL
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS changed_sessions_by_seq ON changed_sessions (seq);

INSERT INTO meta (key, value) VALUES ('change_seq', 0) ON CONFLICT (key) DO NOTHING;

CREATE TRIGGER IF NOT EXISTS changed_on_insert AFTER INSERT ON messages BEGIN
  UPDATE meta SET value = value + 1 WHERE key = 'change_seq';
  INSERT INTO changed_messages (message_id, seq) SELECT NEW.id, value FROM meta WHERE key = 'change_seq'
    ON CONFLICT (message_id) DO UPDATE SET seq = excluded.seq;
END;

CREATE TRIGGER IF NOT EXISTS changed_on_update AFTER UPDATE ON messages
WHEN OLD.at IS NOT NEW.at OR OLD.model IS NOT NEW.model OR OLD.speed IS NOT NEW.speed OR OLD.dir_id IS NOT NEW.dir_id
  OR OLD.thread IS NOT NEW.thread OR OLD.input IS NOT NEW.input OR OLD.output IS NOT NEW.output
  OR OLD.cache_create IS NOT NEW.cache_create OR OLD.cache_read IS NOT NEW.cache_read
  OR OLD.cache_create_5m IS NOT NEW.cache_create_5m OR OLD.cache_create_1h IS NOT NEW.cache_create_1h OR OLD.geo IS NOT NEW.geo
  OR OLD.web_search_requests IS NOT NEW.web_search_requests OR OLD.web_fetch_requests IS NOT NEW.web_fetch_requests
  OR OLD.service_tier IS NOT NEW.service_tier
BEGIN
  UPDATE meta SET value = value + 1 WHERE key = 'change_seq';
  INSERT INTO changed_messages (message_id, seq) SELECT NEW.id, value FROM meta WHERE key = 'change_seq'
    ON CONFLICT (message_id) DO UPDATE SET seq = excluded.seq;
  INSERT INTO changed_sessions (session_id, seq)
    SELECT ms.session_id, (SELECT value FROM meta WHERE key = 'change_seq') FROM message_sessions ms WHERE ms.message_id = NEW.id
    ON CONFLICT (session_id) DO UPDATE SET seq = excluded.seq;
END;

CREATE TRIGGER IF NOT EXISTS changed_on_delete AFTER DELETE ON messages BEGIN
  UPDATE meta SET value = value + 1 WHERE key = 'change_seq';
  INSERT INTO changed_messages (message_id, seq) SELECT OLD.id, value FROM meta WHERE key = 'change_seq'
    ON CONFLICT (message_id) DO UPDATE SET seq = excluded.seq;
END;

CREATE TRIGGER IF NOT EXISTS changed_on_link AFTER INSERT ON message_sessions BEGIN
  UPDATE meta SET value = value + 1 WHERE key = 'change_seq';
  INSERT INTO changed_messages (message_id, seq) SELECT NEW.message_id, value FROM meta WHERE key = 'change_seq'
    ON CONFLICT (message_id) DO UPDATE SET seq = excluded.seq;
  INSERT INTO changed_sessions (session_id, seq) SELECT NEW.session_id, value FROM meta WHERE key = 'change_seq'
    ON CONFLICT (session_id) DO UPDATE SET seq = excluded.seq;
END;

CREATE TRIGGER IF NOT EXISTS changed_on_unlink AFTER DELETE ON message_sessions BEGIN
  UPDATE meta SET value = value + 1 WHERE key = 'change_seq';
  INSERT INTO changed_messages (message_id, seq) SELECT OLD.message_id, value FROM meta WHERE key = 'change_seq'
    ON CONFLICT (message_id) DO UPDATE SET seq = excluded.seq;
  INSERT INTO changed_sessions (session_id, seq) SELECT OLD.session_id, value FROM meta WHERE key = 'change_seq'
    ON CONFLICT (session_id) DO UPDATE SET seq = excluded.seq;
END;
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
	// 3 keeps the server tool counts and the service tier, NULL where a
	// line carries none, and reads every transcript again to fill them
	// (item 116).
	3: `ALTER TABLE messages ADD COLUMN web_search_requests INTEGER;
ALTER TABLE messages ADD COLUMN web_fetch_requests INTEGER;
ALTER TABLE messages ADD COLUMN service_tier TEXT;
CREATE TABLE meta (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
UPDATE transcripts SET read_to = 0;`,
}

// Store is an open statistics database.
type Store struct {
	db *sql.DB
	// done and known are the running ingest's progress, in bytes.
	done, known atomic.Int64
	// index finds the transcripts without globbing every project on
	// every pass (item 196).
	index *claude.Index
	// watch is a connection kept open to ask whether another has
	// written since it last asked (item 215).
	watchMu sync.Mutex
	watch   *sql.Conn
}

// DataVersion is a number that moves whenever another connection, in
// this process or another, commits a write to the store, read on a
// connection kept for it. Item 215.
func (s *Store) DataVersion() (int64, error) {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	if s.watch == nil {
		c, err := s.db.Conn(context.Background())
		if err != nil {
			return 0, err
		}
		s.watch = c
	}
	var v int64
	err := s.watch.QueryRowContext(context.Background(), "PRAGMA data_version").Scan(&v)
	return v, err
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
	if err := sound(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=cache_size(-"+strconv.Itoa(CacheKiB)+")")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, index: claude.NewIndex()}
	if err := s.migrate(path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(path string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	switch {
	case version == 0:
		if _, err := tx.Exec(schema); err != nil {
			return err
		}
	case version < Version:
		for v := version + 1; v <= Version; v++ {
			if _, err := tx.Exec(steps[v]); err != nil {
				return fmt.Errorf("%s: bringing the schema to version %d: %w", path, v, err)
			}
		}
	case version > Version:
		var from int
		if err := tx.QueryRow("SELECT value FROM meta WHERE key = 'readable_from'").Scan(&from); err != nil || from > Version {
			return &Refusal{Path: path, Kind: ErrNewer, Detail: fmt.Sprintf("schema version %d, which this build's version %d cannot read: update Botropolis", version, Version)}
		}
	}
	if version <= Version {
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", Version)); err != nil {
			return err
		}
		if err := setMeta(tx, "readable_from", ReadableFrom); err != nil {
			return err
		}
	}
	if version == Version {
		var by int
		if err := tx.QueryRow("SELECT value FROM meta WHERE key = 'written_by'").Scan(&by); err != nil || by < Version {
			// An older build wrote since this one last did, without the
			// columns it does not know: read everything again to fill them.
			if _, err := tx.Exec("UPDATE transcripts SET read_to = 0"); err != nil {
				return err
			}
		}
	}
	if err := setMeta(tx, "written_by", Version); err != nil {
		return err
	}
	if _, err := tx.Exec(movedSchema); err != nil {
		return err
	}
	if _, err := tx.Exec(changeSchema); err != nil {
		return err
	}
	return tx.Commit()
}

// Refusal is a store Open will not touch: Kind is ErrNewer or
// ErrDamaged, and Detail says what is wrong with the file at Path in
// words that end in what to do about it (item 190).
type Refusal struct {
	Path   string
	Kind   error
	Detail string
}

func (r *Refusal) Error() string { return r.Path + " is " + r.Detail }

func (r *Refusal) Unwrap() error { return r.Kind }

// sound refuses a store at this version without the meta table this
// version always writes, before anything opens it to write: Open would
// otherwise switch its journal and leave it changed (item 190).
func sound(path string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := damaged(db); err != nil {
		return &Refusal{Path: path, Kind: ErrDamaged, Detail: fmt.Sprintf("schema version %d but has no meta table, so it is damaged: move it aside and Botropolis starts a new one", Version)}
	}
	return nil
}

// damaged is ErrDamaged for a store at this version with no meta table.
func damaged(db *sql.DB) error {
	var version, meta int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != Version {
		return nil
	}
	if err := db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'meta'").Scan(&meta); err != nil || meta == 1 {
		return nil
	}
	return ErrDamaged
}

func setMeta(tx *sql.Tx, key string, value int) error {
	_, err := tx.Exec("INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

// DB is the database itself, for the packages that read and write it.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error {
	s.watchMu.Lock()
	if s.watch != nil {
		_ = s.watch.Close()
		s.watch = nil
	}
	s.watchMu.Unlock()
	return s.db.Close()
}
