package stats

import "database/sql"

// statements are the ones an ingest runs for every transcript it
// reads, compiled once a store: the driver keeps no cache, and compiling
// them again on every write was an eighth of a settled daemon's CPU on a
// 2,000-session home (item 213).
type statements struct {
	transcriptRow, lastDir                  *sql.Stmt
	unlinkSessions, unlinkMessages          *sql.Stmt
	session, dir, message, link, transcript *sql.Stmt
}

const (
	transcriptRowSQL = "SELECT size, mtime, read_to FROM transcripts WHERE path = ?"
	lastDirSQL       = `SELECT d.path FROM messages m
		JOIN message_sessions ms ON ms.message_id = m.id
		JOIN dirs d ON d.id = m.dir_id
		WHERE ms.session_id = ? AND m.thread = ? ORDER BY m.at DESC LIMIT 1`
	unlinkSessionsSQL = `DELETE FROM message_sessions WHERE session_id = ?
		AND message_id IN (SELECT id FROM messages WHERE thread = ?)`
	unlinkMessagesSQL = `DELETE FROM messages WHERE thread = ?
		AND id NOT IN (SELECT message_id FROM message_sessions)`
	sessionSQL = `INSERT INTO sessions (id, started_at) VALUES (?, ?)
		ON CONFLICT (id) DO UPDATE SET started_at = CASE
			WHEN excluded.started_at IS NULL THEN sessions.started_at
			WHEN sessions.started_at IS NULL THEN excluded.started_at
			ELSE min(sessions.started_at, excluded.started_at) END`
	dirSQL     = "INSERT INTO dirs (path) VALUES (?) ON CONFLICT (path) DO NOTHING"
	messageSQL = `INSERT INTO messages (id, at, model, speed, dir_id, thread, input, output, cache_create, cache_read, cache_create_5m, cache_create_1h, geo, web_search_requests, web_fetch_requests, service_tier)
		VALUES (?, ?, ?, ?, (SELECT id FROM dirs WHERE path = ?), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET at = excluded.at, model = excluded.model, speed = excluded.speed,
			dir_id = excluded.dir_id, thread = excluded.thread, input = excluded.input, output = excluded.output,
			cache_create = excluded.cache_create, cache_read = excluded.cache_read,
			cache_create_5m = excluded.cache_create_5m, cache_create_1h = excluded.cache_create_1h, geo = excluded.geo,
			web_search_requests = excluded.web_search_requests, web_fetch_requests = excluded.web_fetch_requests, service_tier = excluded.service_tier`
	linkSQL       = "INSERT INTO message_sessions (message_id, session_id) VALUES (?, ?) ON CONFLICT DO NOTHING"
	transcriptSQL = `INSERT INTO transcripts (path, session_id, size, mtime, read_to) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (path) DO UPDATE SET session_id = excluded.session_id, size = excluded.size, mtime = excluded.mtime, read_to = excluded.read_to`
)

func (st *statements) each() []struct {
	stmt  **sql.Stmt
	query string
} {
	return []struct {
		stmt  **sql.Stmt
		query string
	}{
		{&st.transcriptRow, transcriptRowSQL}, {&st.lastDir, lastDirSQL},
		{&st.unlinkSessions, unlinkSessionsSQL}, {&st.unlinkMessages, unlinkMessagesSQL},
		{&st.session, sessionSQL}, {&st.dir, dirSQL}, {&st.message, messageSQL},
		{&st.link, linkSQL}, {&st.transcript, transcriptSQL},
	}
}

// prepared is the store's statements, compiled together on first use.
func (s *Store) prepared() (*statements, error) {
	s.stmtMu.Lock()
	defer s.stmtMu.Unlock()
	if s.stmts != nil {
		return s.stmts, nil
	}
	st := &statements{}
	for _, e := range st.each() {
		stmt, err := s.db.Prepare(e.query)
		if err != nil {
			st.close()
			return nil, err
		}
		*e.stmt = stmt
	}
	s.stmts = st
	return st, nil
}

func (st *statements) close() {
	for _, e := range st.each() {
		if *e.stmt != nil {
			_ = (*e.stmt).Close()
		}
	}
}
