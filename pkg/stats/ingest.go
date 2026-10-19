package stats

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	mainThread  = "main"
	agentPrefix = "agent-"
	synthetic   = "<synthetic>"
)

// Ingest reads every main and subagent transcript under projectsDir into
// the store, each from where the last read stopped. A file that shrank or
// whose modification time went backwards is read again from the start and
// its rows rebuilt to match it; a file that is gone keeps its rows.
// Item 110.
func (s *Store) Ingest(projectsDir string, now time.Time) error {
	mains, err := filepath.Glob(filepath.Join(projectsDir, "*", "*.jsonl"))
	if err != nil {
		return err
	}
	agents, err := filepath.Glob(filepath.Join(projectsDir, "*", "*", "subagents", agentPrefix+"*.jsonl"))
	if err != nil {
		return err
	}
	sort.Strings(mains)
	sort.Strings(agents)
	for _, path := range mains {
		if err := s.ingestFile(path, strings.TrimSuffix(filepath.Base(path), ".jsonl"), mainThread); err != nil {
			return err
		}
	}
	for _, path := range agents {
		session := filepath.Base(filepath.Dir(filepath.Dir(path)))
		thread := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), agentPrefix), ".jsonl")
		if err := s.ingestFile(path, session, thread); err != nil {
			return err
		}
	}
	return s.markGone(mains, now)
}

// markGone sets transcript_gone_at on every session whose main transcript
// is no longer on disk, and clears it on every session whose is.
func (s *Store) markGone(mains []string, now time.Time) error {
	present := map[string]bool{}
	for _, path := range mains {
		present[path] = true
	}
	rows, err := s.db.Query("SELECT path, session_id FROM transcripts WHERE path NOT LIKE ?", "%"+string(filepath.Separator)+"subagents"+string(filepath.Separator)+"%")
	if err != nil {
		return err
	}
	var gone, back []string
	for rows.Next() {
		var path, session string
		if err := rows.Scan(&path, &session); err != nil {
			_ = rows.Close()
			return err
		}
		if present[path] {
			back = append(back, session)
		} else {
			gone = append(gone, session)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, session := range gone {
		if _, err := tx.Exec("UPDATE sessions SET transcript_gone_at = ? WHERE id = ? AND transcript_gone_at IS NULL", now.UnixMilli(), session); err != nil {
			return err
		}
	}
	for _, session := range back {
		if _, err := tx.Exec("UPDATE sessions SET transcript_gone_at = NULL WHERE id = ?", session); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type lineJSON struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	CWD       string `json:"cwd"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input       int64  `json:"input_tokens"`
			Output      int64  `json:"output_tokens"`
			CacheCreate int64  `json:"cache_creation_input_tokens"`
			CacheRead   int64  `json:"cache_read_input_tokens"`
			Speed       string `json:"speed"`
			Geo         string `json:"inference_geo"`
			Tier        string `json:"service_tier"`
			Tools       *struct {
				WebSearch *int64 `json:"web_search_requests"`
				WebFetch  *int64 `json:"web_fetch_requests"`
			} `json:"server_tool_use"`
			Creation *struct {
				FiveMinutes int64 `json:"ephemeral_5m_input_tokens"`
				OneHour     int64 `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	} `json:"message"`
}

type message struct {
	id, model, speed, geo, tier, dir      string
	at                                    int64
	input, output, cacheCreate, cacheRead int64
	// split is the cache writes by lifetime, 5 m then 1 h; nil when the
	// line wrote to the cache without saying which.
	split *[2]int64
	// webSearches and webFetches are nil when the line did not count
	// them.
	webSearches, webFetches *int64
}

type read struct {
	messages []message
	earliest int64
	to       int64
}

type transcriptRow struct {
	size, mtime, readTo int64
}

func (s *Store) ingestFile(path, session, thread string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	size, mtime := info.Size(), info.ModTime().UnixMilli()
	row, known, err := s.transcript(path)
	if err != nil {
		return err
	}
	again := known && (size < row.readTo || mtime < row.mtime)
	if known && !again && size == row.readTo && mtime == row.mtime {
		return nil
	}
	from := int64(0)
	if known && !again {
		from = row.readTo
	}
	fallback := ""
	if from > 0 {
		if fallback, err = s.lastDir(session, thread); err != nil {
			return err
		}
	}
	r, err := readFrom(path, from, fallback)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if again {
		if err := unlink(tx, session, thread); err != nil {
			return err
		}
	}
	if err := upsertSession(tx, session, r.earliest, thread == mainThread); err != nil {
		return err
	}
	for _, m := range r.messages {
		if err := upsertMessage(tx, session, thread, m); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO transcripts (path, session_id, size, mtime, read_to) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (path) DO UPDATE SET session_id = excluded.session_id, size = excluded.size, mtime = excluded.mtime, read_to = excluded.read_to`,
		path, session, size, mtime, r.to); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) transcript(path string) (transcriptRow, bool, error) {
	var row transcriptRow
	err := s.db.QueryRow("SELECT size, mtime, read_to FROM transcripts WHERE path = ?", path).Scan(&row.size, &row.mtime, &row.readTo)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false, nil
	}
	return row, err == nil, err
}

func (s *Store) lastDir(session, thread string) (string, error) {
	var dir string
	err := s.db.QueryRow(`SELECT d.path FROM messages m
		JOIN message_sessions ms ON ms.message_id = m.id
		JOIN dirs d ON d.id = m.dir_id
		WHERE ms.session_id = ? AND m.thread = ? ORDER BY m.at DESC LIMIT 1`, session, thread).Scan(&dir)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return dir, err
}

func readFrom(path string, from int64, cwd string) (read, error) {
	r := read{to: from}
	f, err := os.Open(path)
	if err != nil {
		return r, err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(from, io.SeekStart); err != nil {
		return r, err
	}
	byID := map[string]int{}
	reader := bufio.NewReader(f)
	for {
		raw, err := reader.ReadBytes('\n')
		if err == io.EOF {
			return r, nil
		}
		if err != nil {
			return r, err
		}
		r.to += int64(len(raw))
		var line lineJSON
		if json.Unmarshal(raw, &line) != nil {
			continue
		}
		at, ok := parseTime(line.Timestamp)
		if ok && (r.earliest == 0 || at < r.earliest) {
			r.earliest = at
		}
		if line.CWD != "" {
			cwd = line.CWD
		}
		m, ok := messageOf(line, at, ok, cwd)
		if !ok {
			continue
		}
		if i, seen := byID[m.id]; seen {
			r.messages[i] = m
			continue
		}
		byID[m.id] = len(r.messages)
		r.messages = append(r.messages, m)
	}
}

func messageOf(line lineJSON, at int64, timed bool, cwd string) (message, bool) {
	u := line.Message.Usage
	if line.Type != "assistant" || line.Message.ID == "" || line.Message.Model == synthetic || u == nil || !timed || cwd == "" {
		return message{}, false
	}
	m := message{
		id: line.Message.ID, model: line.Message.Model, speed: u.Speed, geo: u.Geo, tier: u.Tier, dir: cwd, at: at,
		input: u.Input, output: u.Output, cacheCreate: u.CacheCreate, cacheRead: u.CacheRead,
	}
	if u.Tools != nil {
		m.webSearches, m.webFetches = u.Tools.WebSearch, u.Tools.WebFetch
	}
	switch {
	case u.Creation != nil:
		m.split = &[2]int64{u.Creation.FiveMinutes, u.Creation.OneHour}
	case u.CacheCreate == 0:
		m.split = &[2]int64{0, 0}
	}
	return m, true
}

func parseTime(raw string) (int64, bool) {
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return 0, false
	}
	return at.UnixMilli(), true
}

func unlink(tx *sql.Tx, session, thread string) error {
	if _, err := tx.Exec(`DELETE FROM message_sessions WHERE session_id = ?
		AND message_id IN (SELECT id FROM messages WHERE thread = ?)`, session, thread); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM messages WHERE thread = ?
		AND id NOT IN (SELECT message_id FROM message_sessions)`, thread)
	return err
}

func upsertSession(tx *sql.Tx, session string, earliest int64, main bool) error {
	var started any
	if main && earliest > 0 {
		started = earliest
	}
	_, err := tx.Exec(`INSERT INTO sessions (id, started_at) VALUES (?, ?)
		ON CONFLICT (id) DO UPDATE SET started_at = CASE
			WHEN excluded.started_at IS NULL THEN sessions.started_at
			WHEN sessions.started_at IS NULL THEN excluded.started_at
			ELSE min(sessions.started_at, excluded.started_at) END`, session, started)
	return err
}

func upsertMessage(tx *sql.Tx, session, thread string, m message) error {
	if _, err := tx.Exec("INSERT INTO dirs (path) VALUES (?) ON CONFLICT (path) DO NOTHING", m.dir); err != nil {
		return err
	}
	var fiveMinutes, oneHour any
	if m.split != nil {
		fiveMinutes, oneHour = m.split[0], m.split[1]
	}
	if _, err := tx.Exec(`INSERT INTO messages (id, at, model, speed, dir_id, thread, input, output, cache_create, cache_read, cache_create_5m, cache_create_1h, geo, web_search_requests, web_fetch_requests, service_tier)
		VALUES (?, ?, ?, ?, (SELECT id FROM dirs WHERE path = ?), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET at = excluded.at, model = excluded.model, speed = excluded.speed,
			dir_id = excluded.dir_id, thread = excluded.thread, input = excluded.input, output = excluded.output,
			cache_create = excluded.cache_create, cache_read = excluded.cache_read,
			cache_create_5m = excluded.cache_create_5m, cache_create_1h = excluded.cache_create_1h, geo = excluded.geo,
			web_search_requests = excluded.web_search_requests, web_fetch_requests = excluded.web_fetch_requests, service_tier = excluded.service_tier`,
		m.id, m.at, m.model, orNull(m.speed), m.dir, thread, m.input, m.output, m.cacheCreate, m.cacheRead, fiveMinutes, oneHour, orNull(m.geo), m.webSearches, m.webFetches, orNull(m.tier)); err != nil {
		return err
	}
	_, err := tx.Exec("INSERT INTO message_sessions (message_id, session_id) VALUES (?, ?) ON CONFLICT DO NOTHING", m.id, session)
	return err
}

// orNull is an empty string as SQL NULL: a field the line did not carry.
func orNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}
