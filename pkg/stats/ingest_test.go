package stats_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

type usage struct {
	Input       int64  `json:"input_tokens"`
	Output      int64  `json:"output_tokens"`
	CacheCreate int64  `json:"cache_creation_input_tokens"`
	CacheRead   int64  `json:"cache_read_input_tokens"`
	Speed       string `json:"speed,omitempty"`
	Creation    *split `json:"cache_creation,omitempty"`
	Geo         string `json:"inference_geo,omitempty"`
	Tools       *tools `json:"server_tool_use,omitempty"`
	Tier        string `json:"service_tier,omitempty"`
}

type tools struct {
	WebSearch int64 `json:"web_search_requests"`
	WebFetch  int64 `json:"web_fetch_requests"`
}

type split struct {
	FiveMinutes int64 `json:"ephemeral_5m_input_tokens"`
	OneHour     int64 `json:"ephemeral_1h_input_tokens"`
}

type reply struct {
	session, id, at, model, cwd string
	usage                       usage
}

func (r reply) line() string {
	if r.model == "" {
		r.model = "claude-opus-5"
	}
	rec := map[string]any{
		"type":      "assistant",
		"timestamp": r.at,
		"sessionId": r.session,
		"message": map[string]any{
			"id":      r.id,
			"model":   r.model,
			"role":    "assistant",
			"content": []any{map[string]any{"type": "text", "text": "hello"}},
			"usage":   r.usage,
		},
	}
	if r.cwd != "" {
		rec["cwd"] = r.cwd
	}
	data, _ := json.Marshal(rec)
	return string(data)
}

func prompt(session, cwd, at string) string {
	data, _ := json.Marshal(map[string]any{"type": "user", "timestamp": at, "sessionId": session, "cwd": cwd,
		"message": map[string]any{"role": "user", "content": "hi"}})
	return string(data)
}

func ms(t *testing.T, at string) int64 {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, at)
	require.NoError(t, err)
	return parsed.UnixMilli()
}

type fixture struct {
	t        *testing.T
	projects string
	store    *stats.Store
	db       string
	now      time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := filepath.Join(t.TempDir(), "stats.db")
	return &fixture{t: t, projects: filepath.Join(t.TempDir(), "projects"), store: open(t, db), db: db, now: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
}

func (f *fixture) transcriptPath(session string) string {
	return filepath.Join(f.projects, "-w-lib", session+".jsonl")
}

func (f *fixture) write(path string, lines ...string) string {
	f.t.Helper()
	require.NoError(f.t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(f.t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	return path
}

func (f *fixture) appendRaw(path, raw string) {
	f.t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(f.t, err)
	_, err = file.WriteString(raw)
	require.NoError(f.t, err)
	require.NoError(f.t, file.Close())
}

func (f *fixture) transcript(session string, lines ...string) string {
	f.t.Helper()
	return f.write(f.transcriptPath(session), lines...)
}

func (f *fixture) subagent(session, agent string, lines ...string) string {
	f.t.Helper()
	return f.write(filepath.Join(f.projects, "-w-lib", session, "subagents", "agent-"+agent+".jsonl"), lines...)
}

func (f *fixture) ingest() {
	f.t.Helper()
	require.NoError(f.t, f.store.Ingest(f.projects, f.now, nil))
}

func (f *fixture) count(query string, args ...any) int {
	var n int
	_ = f.store.DB().QueryRow(query, args...).Scan(&n)
	return n
}

func (f *fixture) text(query string, args ...any) sql.NullString {
	var s sql.NullString
	_ = f.store.DB().QueryRow(query, args...).Scan(&s)
	return s
}

func (f *fixture) number(query string, args ...any) int64 {
	var n int64
	_ = f.store.DB().QueryRow(query, args...).Scan(&n)
	return n
}

func (f *fixture) messages() int { return f.count("SELECT count(*) FROM messages") }

func (f *fixture) links() int { return f.count("SELECT count(*) FROM message_sessions") }

func (f *fixture) model(id string) string {
	return f.text("SELECT model FROM messages WHERE id = ?", id).String
}

func (f *fixture) dir(id string) string {
	return f.text("SELECT d.path FROM messages m JOIN dirs d ON d.id = m.dir_id WHERE m.id = ?", id).String
}

func (f *fixture) at(id string) int64 { return f.number("SELECT at FROM messages WHERE id = ?", id) }

func (f *fixture) has(id string) bool {
	return f.count("SELECT count(*) FROM messages WHERE id = ?", id) == 1
}

const (
	sessionA = "aaaaaaaa-0000-0000-0000-000000000001"
	sessionB = "bbbbbbbb-0000-0000-0000-000000000002"
	lib      = "/w/lib"
)

func TestIngest(t *testing.T) {
	t.Run("when a session has two messages", func(t *testing.T) {
		f := newFixture(t)
		path := f.transcript(sessionA,
			prompt(sessionA, lib, "2026-10-18T07:59:00.000Z"),
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Input: 3, Output: 5, CacheCreate: 7, CacheRead: 11}}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib, usage: usage{Input: 1, Output: 1}}.line(),
		)
		f.ingest()

		t.Run("it should store two messages", func(t *testing.T) {
			assert.Equal(t, 2, f.messages())
		})

		t.Run("it should store each at its line's time, in unix ms", func(t *testing.T) {
			assert.Equal(t, ms(t, "2026-10-18T08:00:00.000Z"), f.at("msg_1"))
		})

		for _, counter := range []struct {
			column string
			want   int64
		}{{"input", 3}, {"output", 5}, {"cache_create", 7}, {"cache_read", 11}} {
			t.Run("it should store "+counter.column, func(t *testing.T) {
				assert.Equal(t, counter.want, f.number("SELECT "+counter.column+" FROM messages WHERE id = 'msg_1'"))
			})
		}

		t.Run("it should store the line's directory", func(t *testing.T) {
			assert.Equal(t, lib, f.dir("msg_1"))
		})

		t.Run("it should link each message to the session", func(t *testing.T) {
			assert.Equal(t, 2, f.count("SELECT count(*) FROM message_sessions WHERE session_id = ?", sessionA))
		})

		t.Run("it should set the session's started_at to its first line's time", func(t *testing.T) {
			assert.Equal(t, ms(t, "2026-10-18T07:59:00.000Z"), f.number("SELECT started_at FROM sessions WHERE id = ?", sessionA))
		})

		t.Run("it should record read_to as the file's size", func(t *testing.T) {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, info.Size(), f.number("SELECT read_to FROM transcripts WHERE path = ?", path))
		})
	})

	t.Run("when a fork holds the same message as its parent", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.transcript(sessionB, reply{session: sessionB, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.ingest()

		t.Run("it should store the message once", func(t *testing.T) {
			assert.Equal(t, 1, f.messages())
		})

		t.Run("it should link it to both sessions", func(t *testing.T) {
			assert.Equal(t, 2, f.count("SELECT count(*) FROM message_sessions WHERE message_id = 'msg_1'"))
		})
	})

	t.Run("when the model changes mid-session", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, model: "claude-haiku-5"}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib, model: "claude-sonnet-5"}.line(),
		)
		f.ingest()

		t.Run("it should credit each message to the model that answered it", func(t *testing.T) {
			assert.Equal(t, []string{"claude-haiku-5", "claude-sonnet-5"}, []string{f.model("msg_1"), f.model("msg_2")})
		})
	})

	t.Run("when one turn fell back to a backup model", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, model: "claude-opus-5"}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib, model: "claude-sonnet-5"}.line(),
			reply{session: sessionA, id: "msg_3", at: "2026-10-18T08:02:00.000Z", cwd: lib, model: "claude-opus-5"}.line(),
		)
		f.ingest()

		t.Run("it should credit that turn to the backup model", func(t *testing.T) {
			assert.Equal(t, "claude-sonnet-5", f.model("msg_2"))
		})

		t.Run("and it should credit the next turn to the main model", func(t *testing.T) {
			assert.Equal(t, "claude-opus-5", f.model("msg_3"))
		})
	})

	t.Run("when one message streams over several lines", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 1}}.line(),
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:01.000Z", cwd: lib, usage: usage{Output: 5}}.line(),
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:02.000Z", cwd: lib, usage: usage{Output: 9}}.line(),
		)
		f.ingest()

		t.Run("it should store it once", func(t *testing.T) {
			assert.Equal(t, 1, f.messages())
		})

		t.Run("it should keep the last line's usage, unlike ReadTranscript, which keeps the first", func(t *testing.T) {
			assert.Equal(t, int64(9), f.number("SELECT output FROM messages WHERE id = 'msg_1'"))
		})
	})

	t.Run("when the directory changes mid-session", func(t *testing.T) {
		f := newFixture(t)
		worktree := "/w/lib/.claude/worktrees/item-1"
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: worktree}.line(),
		)
		f.ingest()

		t.Run("it should file each message under its own directory", func(t *testing.T) {
			assert.Equal(t, []string{lib, worktree}, []string{f.dir("msg_1"), f.dir("msg_2")})
		})
	})

	t.Run("when a session spans a week", func(t *testing.T) {
		f := newFixture(t)
		first, last := "2026-10-11T08:00:00.000Z", "2026-10-18T08:00:00.000Z"
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: first, cwd: lib}.line(),
			reply{session: sessionA, id: "msg_2", at: last, cwd: lib}.line(),
		)
		f.ingest()

		t.Run("it should keep its first message at its own time", func(t *testing.T) {
			assert.Equal(t, ms(t, first), f.at("msg_1"))
		})

		t.Run("it should keep its last message at its own time", func(t *testing.T) {
			assert.Equal(t, ms(t, last), f.at("msg_2"))
		})
	})

	t.Run("when a message's model is <synthetic>", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, model: "<synthetic>"}.line())
		f.ingest()

		t.Run("it should not store it", func(t *testing.T) {
			assert.Equal(t, 0, f.messages())
		})
	})

	t.Run("when a message has no id", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.ingest()

		t.Run("it should not store it", func(t *testing.T) {
			assert.Equal(t, 0, f.messages())
		})
	})

	t.Run("when a line has no cwd", func(t *testing.T) {
		t.Run("and an earlier line had one", func(t *testing.T) {
			f := newFixture(t)
			f.transcript(sessionA,
				prompt(sessionA, lib, "2026-10-18T07:59:00.000Z"),
				reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z"}.line(),
			)
			f.ingest()

			t.Run("it should use that one", func(t *testing.T) {
				assert.Equal(t, lib, f.dir("msg_1"))
			})
		})

		t.Run("and the earlier line was read by an earlier ingest", func(t *testing.T) {
			f := newFixture(t)
			path := f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
			f.ingest()
			f.appendRaw(path, reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z"}.line()+"\n")
			f.ingest()

			t.Run("it should use the directory of the session's last stored message", func(t *testing.T) {
				assert.Equal(t, lib, f.dir("msg_2"))
			})
		})

		t.Run("and no earlier line had one", func(t *testing.T) {
			f := newFixture(t)
			f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z"}.line())
			f.ingest()

			t.Run("it should not store it", func(t *testing.T) {
				assert.Equal(t, 0, f.messages())
			})
		})
	})

	t.Run("when a subagent answers", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, prompt(sessionA, lib, "2026-10-18T07:59:00.000Z"))
		f.subagent(sessionA, "a1b2c3", reply{session: sessionA, id: "msg_s", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.ingest()

		t.Run("it should store its message under the parent session", func(t *testing.T) {
			assert.Equal(t, sessionA, f.text("SELECT session_id FROM message_sessions WHERE message_id = 'msg_s'").String)
		})

		t.Run("it should give it the agent's thread", func(t *testing.T) {
			assert.Equal(t, "a1b2c3", f.text("SELECT thread FROM messages WHERE id = 'msg_s'").String)
		})
	})

	t.Run("when usage.speed is given", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Speed: "fast"}}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib}.line(),
		)
		f.ingest()

		t.Run("it should keep it", func(t *testing.T) {
			assert.Equal(t, "fast", f.text("SELECT speed FROM messages WHERE id = 'msg_1'").String)
		})

		t.Run("and when it is absent, it should leave it empty", func(t *testing.T) {
			assert.False(t, f.text("SELECT speed FROM messages WHERE id = 'msg_2'").Valid)
		})
	})

	t.Run("when the main thread's messages are stored", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.ingest()

		t.Run("it should give them the thread main", func(t *testing.T) {
			assert.Equal(t, "main", f.text("SELECT thread FROM messages WHERE id = 'msg_1'").String)
		})
	})

	t.Run("when a line is not JSON", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line(),
			"{not json",
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib}.line(),
		)
		f.ingest()

		t.Run("it should store the lines around it", func(t *testing.T) {
			assert.Equal(t, 2, f.messages())
		})
	})

	t.Run("when ingest runs twice with nothing new", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib}.line(),
		)
		f.ingest()
		f.ingest()

		t.Run("it should store nothing twice", func(t *testing.T) {
			assert.Equal(t, 2, f.links())
		})
	})

	t.Run("when lines are appended", func(t *testing.T) {
		f := newFixture(t)
		first := reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Input: 1}}
		path := f.transcript(sessionA, first.line())
		f.ingest()
		rewritten := first
		rewritten.usage.Input = 9
		f.write(path, rewritten.line(), reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib}.line())
		f.ingest()

		t.Run("it should store the new message", func(t *testing.T) {
			assert.True(t, f.has("msg_2"))
		})

		t.Run("it should not read the old lines again", func(t *testing.T) {
			assert.Equal(t, int64(1), f.number("SELECT input FROM messages WHERE id = 'msg_1'"))
		})
	})

	t.Run("when the last line is not finished", func(t *testing.T) {
		f := newFixture(t)
		second := reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib}.line()
		half := len(second) / 2
		path := f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.appendRaw(path, second[:half])
		f.ingest()

		t.Run("it should not store it yet", func(t *testing.T) {
			assert.False(t, f.has("msg_2"))
		})

		t.Run("and when it is finished later", func(t *testing.T) {
			f.appendRaw(path, second[half:]+"\n")
			f.ingest()

			t.Run("it should store it", func(t *testing.T) {
				assert.True(t, f.has("msg_2"))
			})
		})
	})

	t.Run("when the file shrinks", func(t *testing.T) {
		f := newFixture(t)
		kept := reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line()
		path := f.transcript(sessionA, kept, reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib}.line())
		f.ingest()
		f.write(path, kept)
		f.ingest()

		t.Run("it should drop a message the file no longer holds", func(t *testing.T) {
			assert.False(t, f.has("msg_2"))
		})
	})

	t.Run("when the file's mtime goes backwards at the same size", func(t *testing.T) {
		f := newFixture(t)
		first := reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Input: 1}}
		path := f.transcript(sessionA, first.line())
		f.ingest()
		rewritten := first
		rewritten.usage.Input = 9
		f.write(path, rewritten.line())
		earlier := time.Now().Add(-48 * time.Hour)
		require.NoError(t, os.Chtimes(path, earlier, earlier))
		f.ingest()

		t.Run("it should read it again from the start", func(t *testing.T) {
			assert.Equal(t, int64(9), f.number("SELECT input FROM messages WHERE id = 'msg_1'"))
		})
	})

	t.Run("when the transcript is deleted", func(t *testing.T) {
		t.Run("and ingest runs again", func(t *testing.T) {
			f := newFixture(t)
			path := f.transcript(sessionA,
				reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line(),
				reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib}.line(),
			)
			f.ingest()
			require.NoError(t, os.Remove(path))
			f.ingest()

			t.Run("it should keep every message it had stored", func(t *testing.T) {
				assert.Equal(t, 2, f.messages())
			})

			t.Run("it should keep the session's links", func(t *testing.T) {
				assert.Equal(t, 2, f.links())
			})
		})
	})
}
