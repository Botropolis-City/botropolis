package state_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 205: a load reuses what it read and built before wherever the
// watcher says nothing changed, so at every step of a replay it is held
// to what a loader starting from nothing loads from the same home. The
// replay tells the incremental loader what the daemon's watcher would:
// every file written and every directory it holds.
func TestLoadsEqualFromScratch(t *testing.T) {
	const (
		steps   = 150
		stopped = 30
		live    = 3
	)
	dirs := []string{"/w/oracle-a", "/w/oracle-b", "/w/oracle-c"}
	for _, seed := range []int64{205, 206, 207} {
		t.Run(fmt.Sprintf("when a home of %d stopped and %d live sessions is replayed for %d steps, with seed %d", stopped, live, steps, seed), func(t *testing.T) {
			r := rand.New(rand.NewSource(seed))
			at := now
			stamp := func() string { return at.Format("2006-01-02T15:04:05.000Z") }
			home := helpers.NewHome(t)
			probes := state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }}
			incremental := state.NewLoader(home.Path, probes)
			type session struct {
				id, cwd, path string
				pid           int
				live, gone    bool
			}
			var sessions []*session
			next := 0
			add := func(isLive bool) *session {
				next++
				s := &session{id: fmt.Sprintf("00000000-0000-4205-9000-%012d", next), cwd: dirs[r.Intn(len(dirs))], pid: 50000 + next}
				s.path = home.Transcript(s.id, s.cwd, helpers.UserPrompt(s.id, s.cwd, stamp()), helpers.AssistantReply(s.id, fmt.Sprintf("msg_%d_0", next), stamp()))
				incremental.Changed(s.path)
				if isLive {
					home.Session(s.pid, s.id, s.cwd, "interactive", "busy")
					s.live = true
				}
				sessions = append(sessions, s)
				return s
			}
			for i := 0; i < stopped; i++ {
				add(false)
			}
			for i := 0; i < live; i++ {
				add(true)
			}
			pick := func(wantLive bool) *session {
				var from []*session
				for _, s := range sessions {
					if !s.gone && s.live == wantLive {
						from = append(from, s)
					}
				}
				if len(from) == 0 {
					return nil
				}
				return from[r.Intn(len(from))]
			}
			record := func(s *session) string {
				return filepath.Join(home.ClaudeDir(), "sessions", fmt.Sprintf("%d.json", s.pid))
			}
			actions := []struct {
				name string
				do   func()
			}{
				{"a stopped session gets a reply", func() {
					if s := pick(false); s != nil {
						home.AppendTranscript(s.id, s.cwd, helpers.AssistantReply(s.id, fmt.Sprintf("msg_%s_%d", s.id[24:], r.Int()), stamp()))
						incremental.Changed(s.path)
					}
				}},
				{"a live session gets a reply", func() {
					if s := pick(true); s != nil {
						home.AppendTranscript(s.id, s.cwd, helpers.AssistantReply(s.id, fmt.Sprintf("msg_%s_%d", s.id[24:], r.Int()), stamp()))
						incremental.Changed(s.path)
					}
				}},
				{"a stopped session is titled", func() {
					if s := pick(false); s != nil {
						home.AppendTranscript(s.id, s.cwd, helpers.AITitle(s.id, fmt.Sprintf("Job %d", r.Intn(1000))))
						incremental.Changed(s.path)
					}
				}},
				{"a new session stops at once", func() { add(false) }},
				{"a new session starts", func() { add(true) }},
				{"a stopped transcript is deleted", func() {
					if s := pick(false); s != nil {
						require.NoError(t, os.Remove(s.path))
						s.gone = true
						incremental.Changed(s.path)
					}
				}},
				{"a live session stops", func() {
					if s := pick(true); s != nil {
						home.RemoveSession(s.pid)
						s.live = false
						incremental.Changed(record(s))
					}
				}},
				{"a stopped session resumes", func() {
					if s := pick(false); s != nil {
						home.Session(s.pid, s.id, s.cwd, "interactive", "busy")
						s.live = true
						incremental.Changed(record(s))
					}
				}},
				{"a stopped session's subagent works elsewhere, and the session replies", func() {
					if s := pick(false); s != nil {
						agent := fmt.Sprintf("a%d", r.Intn(1000))
						sub := home.Subagent(s.id, s.cwd, agent, "", helpers.UserPrompt(s.id, "/w/elsewhere", stamp()))
						home.AppendTranscript(s.id, s.cwd, helpers.AssistantReply(s.id, fmt.Sprintf("msg_%s_%d", s.id[24:], r.Int()), stamp()))
						incremental.Changed(sub)
						incremental.Changed(s.path)
					}
				}},
				{"a prompt is typed", func() {
					if s := pick(true); s != nil {
						home.Prompt(s.id, s.cwd, fmt.Sprintf("please fix oracle%d", r.Intn(50)), at)
						incremental.Changed(filepath.Join(home.ClaudeDir(), "history.jsonl"))
					}
				}},
				{"nothing happens", func() {}},
			}
			watchAll := func() {
				for _, dir := range incremental.WatchDirs() {
					incremental.Watching(dir, true)
				}
			}
			_, err := incremental.Load(at)
			require.NoError(t, err)
			watchAll()

			t.Run("it should load at every step what a loader starting from nothing loads", func(t *testing.T) {
				for step := 0; step < steps; step++ {
					at = at.Add(time.Second)
					action := actions[r.Intn(len(actions))]
					action.do()
					got, err := incremental.Load(at)
					require.NoError(t, err)
					watchAll()
					want, err := state.NewLoader(home.Path, probes).Load(at)
					require.NoError(t, err)
					if !assert.Equal(t, want, got, "step %d, after %q", step, action.name) {
						return
					}
				}
			})
		})
	}
}
