package demo

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTeamScript(t *testing.T) {
	t.Run("when a team session has more than one prompt", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tidepool.yaml")
		writeFile(t, path, "project: tidepool\nsessions:\n  - label: crew\n    model: sonnet\n    budget_usd: 1\n    team: true\n    prompts: [one, two]\n")
		_, err := LoadScript(path)

		t.Run("it should refuse it, since a team is recorded from one interactive prompt", func(t *testing.T) {
			assert.ErrorContains(t, err, "one prompt")
		})
	})
}

// fakeTerminal stands in for claude under tmux: it writes the lead's
// and its teammate's transcripts into the home as Claude Code would,
// then keeps the teammate writing for as long as busy says.
type fakeTerminal struct {
	t                 *testing.T
	home, lead        string
	dir               string
	args, env         []string
	busy              time.Duration
	usage, mateUsage  string
	interrupt         func()
	clock             *fakeTeamClock
	mu                sync.Mutex
	stopped           bool
	stoppedAt, lastAt time.Time
}

func (f *fakeTerminal) run(_ context.Context, dir string, args, env []string) (Run, error) {
	f.dir, f.args, f.env = dir, args, env
	team := "session-2aa29be8"
	folder := filepath.Join(f.home, ".claude", "projects", "-home-demo-src-tidepool")
	other := filepath.Join(f.home, ".claude", "projects", "-home-demo-src-kiln")
	lead := `{"type":"user","sessionId":"` + f.lead + `","cwd":"/home/demo/src/tidepool","teamName":"` + team + `","timestamp":"2026-10-25T10:00:00.000Z"}` + "\n" +
		`{"type":"assistant","sessionId":"` + f.lead + `","cwd":"/home/demo/src/tidepool","teamName":"` + team + `","timestamp":"2026-10-25T10:00:09.000Z","message":{"id":"msg_l","model":"claude-haiku-4-5","stop_reason":"end_turn","usage":` + f.usage + `,"content":[{"type":"text","text":"done"}]}}` + "\n"
	writeFile(f.t, filepath.Join(folder, f.lead+".jsonl"), lead)
	writeFile(f.t, filepath.Join(folder, f.lead, "subagents", "agent-a1.jsonl"), `{"type":"assistant"}`+"\n")
	// The teammate carries no team name the recorder knows, and is filed
	// below its project folder rather than beside its lead, as an
	// in-process teammate may be.
	matePath := filepath.Join(other, "team", "mate-1.jsonl")
	mate := func() string {
		return `{"type":"user","sessionId":"mate-1","cwd":"/home/demo/src/kiln","agentName":"reader","timestamp":"2026-10-25T10:00:01.000Z"}` + "\n" +
			`{"type":"assistant","sessionId":"mate-1","cwd":"/home/demo/src/kiln","timestamp":"2026-10-25T10:00:02.000Z","message":{"id":"msg_m","model":"claude-haiku-4-5","stop_reason":"end_turn","usage":` + f.mateUsage + `,"content":[{"type":"text","text":"sent"}]}}` + "\n" +
			`{"type":"attachment","attachment":{"type":"prompt_snapshot"}}` + "\n"
	}
	writeFile(f.t, matePath, mate())
	f.lastAt = f.now()
	if f.clock != nil {
		grown := mate()
		f.clock.onWait = func(polls int) {
			if polls > f.clock.writes {
				return
			}
			grown += `{"type":"user","sessionId":"mate-1"}` + "\n"
			writeFile(f.t, matePath, grown)
			f.lastAt = f.now()
		}
	}
	if f.busy > 0 {
		go func() {
			for end := time.Now().Add(f.busy); time.Now().Before(end); time.Sleep(2 * time.Millisecond) {
				f.mu.Lock()
				if f.stopped {
					f.mu.Unlock()
					return
				}
				writeFile(f.t, matePath, mate()+`{"type":"user","sessionId":"mate-1"}`+"\n")
				f.lastAt = time.Now()
				f.mu.Unlock()
			}
		}()
	}
	if f.interrupt != nil {
		go f.interrupt()
	}
	return f, nil
}

func (f *fakeTerminal) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped, f.stoppedAt = true, f.now()
	return nil
}

// teamClock is the clock the recorder runs on: the fake one when the
// terminal has it, and the real one when not.
func (f *fakeTerminal) teamClock() TeamClock {
	if f.clock != nil {
		return f.clock
	}
	return nil
}

func (f *fakeTerminal) now() time.Time {
	if f.clock != nil {
		return f.clock.Now()
	}
	return time.Now()
}

// fakeTeamClock is a recording's clock that moves only when the recorder
// waits for its next poll, by one poll each time, and lets the terminal
// write on the first writes polls.
type fakeTeamClock struct {
	at     time.Time
	polls  int
	writes int
	onWait func(polls int)
}

func (c *fakeTeamClock) Now() time.Time { return c.at }

func (c *fakeTeamClock) Wait(ctx context.Context, d time.Duration) error {
	c.at = c.at.Add(d)
	c.polls++
	if c.onWait != nil {
		c.onWait(c.polls)
	}
	return ctx.Err()
}

func TestRecordTeam(t *testing.T) {
	script := Script{Project: "tidepool", Sessions: []ScriptedSession{
		{Label: "crew", Model: "haiku", BudgetUSD: 1, Team: true, Prompts: []string{"spawn a reader"}},
	}}
	lead := SessionID("tidepool", "crew")
	cheap := `{"input_tokens":10,"output_tokens":10}`
	limits := TeamLimits{Poll: time.Millisecond, Quiet: 30 * time.Millisecond, Time: 2 * time.Second}
	record := func(t *testing.T, f *fakeTerminal, ctx context.Context) (string, error) {
		t.Helper()
		home, corpus := t.TempDir(), t.TempDir()
		f.t, f.home, f.lead = t, home, lead
		if f.usage == "" {
			f.usage = cheap
		}
		if f.mateUsage == "" {
			f.mateUsage = cheap
		}
		stranger := filepath.Join(home, ".claude", "projects", "-home-demo-src-tidepool", "stranger.jsonl")
		writeFile(t, stranger, `{"type":"user","sessionId":"stranger","cwd":"/home/demo/src/tidepool"}`+"\n")
		before := time.Now().Add(-time.Hour)
		require.NoError(t, os.Chtimes(stranger, before, before))
		r := Recorder{Home: home, Src: Root, Corpus: corpus, MCPConfig: "/m.json", Terminal: f.run, Team: limits, TeamClock: f.teamClock(),
			Claude: func(context.Context, string, []string) ([]byte, error) {
				t.Fatal("a team session ran with -p")
				return nil, nil
			}}
		_, err := r.Record(ctx, script)
		return corpus, err
	}

	t.Run("when a team session is recorded", func(t *testing.T) {
		f := &fakeTerminal{}
		corpus, err := record(t, f, context.Background())
		require.NoError(t, err)

		t.Run("it should run claude through the terminal, never with -p", func(t *testing.T) {
			assert.NotContains(t, f.args, "-p")
		})
		t.Run("it should start the lead under its own session id", func(t *testing.T) {
			i := slices.Index(f.args, "--session-id")
			require.GreaterOrEqual(t, i, 0)
			assert.Equal(t, lead, f.args[i+1])
		})
		t.Run("it should turn agent teams on", func(t *testing.T) {
			assert.Contains(t, f.env, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
		})
		t.Run("it should export the lead's transcript", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(corpus, "tidepool", lead+".jsonl"))
		})
		t.Run("it should export the lead's subagents with it", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(corpus, "tidepool", lead, "subagents", "agent-a1.jsonl"))
		})
		t.Run("it should export every transcript the run wrote, keeping its path within its project folder", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(corpus, "kiln", "team", "mate-1.jsonl"))
		})
		t.Run("it should export nothing written before the run started", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(corpus, "tidepool", "stranger.jsonl"))
		})
		t.Run("it should export the teammate through the same scrub", func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(corpus, "kiln", "team", "mate-1.jsonl"))
			require.NoError(t, err)
			assert.NotContains(t, string(data), "prompt_snapshot")
		})
		t.Run("it should stop the terminal once the team is done", func(t *testing.T) {
			assert.True(t, f.stopped)
		})
	})

	t.Run("and a teammate's transcript carries no team name the recorder knows", func(t *testing.T) {
		f := &fakeTerminal{mateUsage: `{"input_tokens":1000000,"output_tokens":1000000}`}
		_, err := record(t, f, context.Background())

		t.Run("it should still count its cost against the budget", func(t *testing.T) {
			assert.ErrorContains(t, err, "over its budget")
		})
	})

	t.Run("and its transcripts cost more than its budget", func(t *testing.T) {
		f := &fakeTerminal{usage: `{"input_tokens":1000000,"output_tokens":1000000}`}
		_, err := record(t, f, context.Background())

		t.Run("it should stop the run", func(t *testing.T) {
			assert.True(t, f.stopped)
		})
		t.Run("it should fail, naming the cost and the budget", func(t *testing.T) {
			assert.Regexp(t, `cost \$[0-9]+\.[0-9]{2}\b.*budget of \$1\.00`, err)
		})
	})

	t.Run("and it has not finished when the time limit passes", func(t *testing.T) {
		saved := limits
		limits.Time = 50 * time.Millisecond
		t.Cleanup(func() { limits = saved })
		f := &fakeTerminal{busy: time.Second}
		_, err := record(t, f, context.Background())

		t.Run("it should stop the run", func(t *testing.T) {
			assert.True(t, f.stopped)
		})
		t.Run("it should fail, saying it ran out of time", func(t *testing.T) {
			assert.ErrorContains(t, err, "ran out of time")
		})
	})

	// Item 254: on the recording's own clock, with a teammate whose
	// transcript grows as a real one does, so a filesystem whose times are
	// coarser than the clock cannot hide a write.
	t.Run("and the lead's turn has ended but a teammate wrote within the quiet period", func(t *testing.T) {
		f := &fakeTerminal{clock: &fakeTeamClock{at: time.Now(), writes: 5}}
		_, err := record(t, f, context.Background())
		require.NoError(t, err)
		quietFor := f.stoppedAt.Sub(f.lastAt)

		t.Run("it should keep running until the teammate has been quiet for the quiet period", func(t *testing.T) {
			assert.GreaterOrEqual(t, quietFor, limits.Quiet)
		})
		t.Run("it should stop on the first poll after the quiet period", func(t *testing.T) {
			assert.Less(t, quietFor, limits.Quiet+limits.Poll)
		})
	})

	t.Run("and the recorder itself is interrupted", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := &fakeTerminal{busy: time.Second, interrupt: func() { time.Sleep(20 * time.Millisecond); cancel() }}
		_, err := record(t, f, ctx)

		t.Run("it should stop the tmux session, so no claude is left spending", func(t *testing.T) {
			assert.True(t, f.stopped)
		})
		t.Run("it should fail, saying it was interrupted", func(t *testing.T) {
			assert.True(t, err != nil && strings.Contains(err.Error(), "interrupted"), "%v", err)
		})
	})
}
