package stats

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const (
	mainThread  = "main"
	agentPrefix = "agent-"
	synthetic   = "<synthetic>"
)

// YieldEvery is how many bytes of a transcript ingest reads between
// yields.
const YieldEvery = 1 << 20

// CheckpointEvery is how many bytes of a transcript ingest reads between
// commits, so a stop part-way through a long one keeps what it read
// (item 159).
const CheckpointEvery = 16 << 20

// Ingest reads every main and subagent transcript under projectsDir into
// the store, each from where the last read stopped. A file that shrank or
// whose modification time went backwards is read again from the start and
// its rows rebuilt to match it; a file that is gone keeps its rows.
// Item 110.
func (s *Store) Ingest(projectsDir string, now time.Time, yield func()) error {
	return s.IngestContext(context.Background(), projectsDir, now, yield)
}

// IngestContext is Ingest, stopping at the next yield once ctx is done
// and returning its error. What it read up to the last checkpoint is
// kept, and the next ingest reads on from there (item 159).
func (s *Store) IngestContext(ctx context.Context, projectsDir string, now time.Time, yield func()) error {
	s.index.Refresh()
	mains := s.index.Transcripts(projectsDir)
	agents := s.index.SubagentTranscripts(projectsDir)
	rows, err := s.transcripts()
	if err != nil {
		return err
	}
	known, err := s.left(rows, mains, agents)
	if err != nil {
		return err
	}
	s.done.Store(0)
	s.known.Store(known)
	if yield == nil {
		yield = func() {}
	}
	step := func() error {
		yield()
		return ctx.Err()
	}
	for _, path := range mains {
		if err := step(); err != nil {
			return err
		}
		if err := s.ingestFile(rows, path, strings.TrimSuffix(filepath.Base(path), ".jsonl"), mainThread, step); err != nil {
			return err
		}
	}
	for _, path := range agents {
		session := filepath.Base(filepath.Dir(filepath.Dir(path)))
		thread := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), agentPrefix), ".jsonl")
		if err := step(); err != nil {
			return err
		}
		if err := s.ingestFile(rows, path, session, thread, step); err != nil {
			return err
		}
	}
	return s.markGone(mains, now)
}

// markGone sets transcript_gone_at on every session none of whose main
// transcripts is on disk any more, and clears it on every session one of
// whose is, writing only the sessions that change (item 196).
func (s *Store) markGone(mains []string, now time.Time) error {
	present := map[string]bool{}
	for _, path := range mains {
		present[path] = true
	}
	rows, err := s.db.Query(`SELECT t.path, t.session_id, s.transcript_gone_at IS NOT NULL FROM transcripts t
		JOIN sessions s ON s.id = t.session_id WHERE t.path NOT LIKE ?`, "%"+string(filepath.Separator)+"subagents"+string(filepath.Separator)+"%")
	if err != nil {
		return err
	}
	there, wasGone := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var path, session string
		var gone bool
		if err := rows.Scan(&path, &session, &gone); err != nil {
			_ = rows.Close()
			return err
		}
		there[session] = there[session] || present[path]
		wasGone[session] = gone
	}
	if err := rows.Close(); err != nil {
		return err
	}
	var gone, back []string
	for session, isThere := range there {
		switch {
		case isThere && wasGone[session]:
			back = append(back, session)
		case !isThere && !wasGone[session]:
			gone = append(gone, session)
		}
	}
	if len(gone) == 0 && len(back) == 0 {
		return nil
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

// readPlan is what an ingest has to read of one transcript: from the
// byte from to size, after dropping its rows when again.
type readPlan struct {
	size, mtime, from int64
	again, skip       bool
}

func (s *Store) plan(rows map[string]transcriptRow, path string) (readPlan, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return readPlan{skip: true}, nil
	}
	if err != nil {
		return readPlan{}, err
	}
	p := readPlan{size: info.Size(), mtime: info.ModTime().UnixMilli()}
	row, known := rows[path]
	p.again = known && (p.size < row.readTo || p.mtime < row.mtime)
	p.skip = known && !p.again && p.size == row.readTo && p.mtime == row.mtime
	if known && !p.again {
		p.from = row.readTo
	}
	return p, nil
}

// left is how many bytes an ingest of paths has to read.
func (s *Store) left(rows map[string]transcriptRow, paths ...[]string) (int64, error) {
	var total int64
	for _, list := range paths {
		for _, path := range list {
			p, err := s.plan(rows, path)
			if err != nil {
				return 0, err
			}
			if !p.skip {
				total += max(p.size-p.from, 0)
			}
		}
	}
	return total, nil
}

// Progress is how many bytes the running or last ingest has read, of
// how many it had to (item 159).
func (s *Store) Progress() (done, known int64) {
	return s.done.Load(), s.known.Load()
}

func (s *Store) ingestFile(rows map[string]transcriptRow, path, session, thread string, step func() error) error {
	p, err := s.plan(rows, path)
	if err != nil || p.skip {
		return err
	}
	size, mtime, from, again := p.size, p.mtime, p.from, p.again
	fallback := ""
	if from > 0 {
		if fallback, err = s.lastDir(session, thread); err != nil {
			return err
		}
	}
	rebuild := again
	save := func(r read) error {
		if err := s.save(path, session, thread, size, mtime, r, rebuild); err != nil {
			return err
		}
		rebuild = false
		return nil
	}
	r, err := readFrom(path, from, fallback, &s.done, step, save)
	if err != nil {
		return err
	}
	return save(r)
}

// save commits what a read has found so far and how far it got, at the
// end of a transcript or at a checkpoint inside one. rebuild drops the
// thread's rows first, for a transcript read again from the start.
func (s *Store) save(path, session, thread string, size, mtime int64, r read, rebuild bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if rebuild {
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

// transcripts is every transcript's row, read once a pass rather than
// once a file (item 196).
func (s *Store) transcripts() (map[string]transcriptRow, error) {
	rows, err := s.db.Query("SELECT path, size, mtime, read_to FROM transcripts")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	all := map[string]transcriptRow{}
	for rows.Next() {
		var path string
		var row transcriptRow
		if err := rows.Scan(&path, &row.size, &row.mtime, &row.readTo); err != nil {
			return nil, err
		}
		all[path] = row
	}
	return all, rows.Err()
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

// readFrom reads path from the byte from, counting every byte into
// done, handing what it has found to checkpoint every CheckpointEvery
// bytes and starting afresh, and stopping when step does.
func readFrom(path string, from int64, cwd string, done *atomic.Int64, step func() error, checkpoint func(read) error) (read, error) {
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
	var buf []byte
	sinceYield, sinceCheckpoint := 0, 0
	for {
		if sinceCheckpoint >= CheckpointEvery {
			if err := checkpoint(r); err != nil {
				return r, err
			}
			r.messages, byID, sinceCheckpoint = nil, map[string]int{}, 0
		}
		raw, kept, err := nextLine(reader, buf)
		buf = kept
		if err == io.EOF {
			return r, nil
		}
		if err != nil {
			return r, err
		}
		r.to += int64(len(raw))
		done.Add(int64(len(raw)))
		sinceCheckpoint += len(raw)
		if sinceYield += len(raw); sinceYield >= YieldEvery {
			if err := step(); err != nil {
				return r, err
			}
			sinceYield = 0
		}
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

// nextLine is the next whole line, read without a fresh allocation: it
// points into the reader's buffer, or into buf when the line is longer,
// and is good until the next call. kept is buf to pass back in.
func nextLine(reader *bufio.Reader, buf []byte) (line, kept []byte, err error) {
	buf = buf[:0]
	for {
		chunk, err := reader.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			buf = append(buf, chunk...)
			continue
		}
		if err != nil {
			return nil, buf, err
		}
		if len(buf) == 0 {
			return chunk, buf, nil
		}
		buf = append(buf, chunk...)
		return buf, buf, nil
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
