package commands_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/control"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sid = "08b1d655-276f-495b-a61f-2214af35a97a"
	cwd = "/home/avesta/workspaces/github/mCedar/cinders"
)

type fakeSource struct {
	snapshot state.Snapshot
	err      error
	direct   bool
}

func (f *fakeSource) Snapshot(direct bool) (state.Snapshot, error) {
	f.direct = direct
	return f.snapshot, f.err
}

type fakeController struct {
	calls  []string
	id     string
	agents []control.Agent
	// liveAgents, when set, is what agents without --all lists.
	liveAgents []control.Agent
	failure    error
}

func (f *fakeController) New(_ context.Context, dir, prompt string) (string, error) {
	f.calls = append(f.calls, "new "+dir+" "+prompt)
	return f.id, f.failure
}

func (f *fakeController) Attach(id string) error {
	f.calls = append(f.calls, "attach "+id)
	return f.failure
}

func (f *fakeController) AttachHere(id string) error {
	f.calls = append(f.calls, "attach here "+id)
	return f.failure
}

func (f *fakeController) Stop(_ context.Context, id string) error {
	f.calls = append(f.calls, "stop "+id)
	return f.failure
}

func (f *fakeController) Remove(_ context.Context, id string) error {
	f.calls = append(f.calls, "rm "+id)
	return f.failure
}

func (f *fakeController) Reveal(dir string) error {
	f.calls = append(f.calls, "reveal "+dir)
	return f.failure
}

func (f *fakeController) Copy(_ context.Context, text string) error {
	f.calls = append(f.calls, "copy "+text)
	return f.failure
}

func (f *fakeController) Resume(_ context.Context, dir, sessionID string) (string, error) {
	f.calls = append(f.calls, "resume "+dir+" "+sessionID)
	return f.id, f.failure
}

func (f *fakeController) Fork(_ context.Context, dir, sessionID string) (string, error) {
	f.calls = append(f.calls, "fork "+dir+" "+sessionID)
	return f.id, f.failure
}

func (f *fakeController) Agents(_ context.Context, all bool) ([]control.Agent, error) {
	f.calls = append(f.calls, fmt.Sprintf("agents all=%v", all))
	if !all && f.liveAgents != nil {
		return f.liveAgents, f.failure
	}
	return f.agents, f.failure
}

func TestStatus(t *testing.T) {
	now := time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)
	snapshot := state.Snapshot{At: now, Sessions: []state.Session{{
		ID: sid, State: state.NeedsYou, Tool: "Bash", CWD: cwd, Title: "Fix the CI queue", Branch: "main",
		Model: "claude-opus-5[1m]", ContextPercent: 25, FreshTokensPerHour: 152_000, CacheReadPerHour: 8_900_000,
		Subagents: 4, SubagentsInFlight: 1, StartedAt: now.Add(-5*time.Hour - 6*time.Minute), LastActivity: now.Add(-6 * time.Minute),
	}}}

	t.Run("when the source has one session", func(t *testing.T) {
		source := &fakeSource{snapshot: snapshot}
		var out bytes.Buffer
		require.NoError(t, commands.NewStatus(source).Run(&out, true, false, false))
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")

		t.Run("it should pass the direct flag through", func(t *testing.T) {
			assert.True(t, source.direct)
		})

		t.Run("it should print a header and one row", func(t *testing.T) {
			assert.Len(t, lines, 2)
		})

		t.Run("it should render every column, ending with how long the session has been idle", func(t *testing.T) {
			assert.Regexp(t, `^needs-you\s+Bash\s+cinders\s+Fix the CI queue\s+main\s+claude-opus-5\[1m\]\s+25%\s+152k\s+8\.9M\s+1/4\s+6m$`, lines[1])
		})

		t.Run("it should head the last column IDLE", func(t *testing.T) {
			assert.Regexp(t, `IDLE$`, lines[0])
		})
	})

	t.Run("when the source holds a resting session", func(t *testing.T) {
		resting := state.Snapshot{At: now, Sessions: append(snapshot.Sessions, state.Session{ID: "old", State: state.Resting, CWD: cwd, Title: "old"})}

		t.Run("and all sessions are not asked for", func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, commands.NewStatus(&fakeSource{snapshot: resting}).Run(&out, true, false, false))

			t.Run("it should leave the resting one out", func(t *testing.T) {
				assert.Len(t, strings.Split(strings.TrimSpace(out.String()), "\n"), 2)
			})
		})

		t.Run("and all sessions are asked for", func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, commands.NewStatus(&fakeSource{snapshot: resting}).Run(&out, true, true, false))

			t.Run("it should print the resting one too", func(t *testing.T) {
				assert.Contains(t, out.String(), "resting")
			})
		})
	})

	t.Run("when the source has no sessions", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, commands.NewStatus(&fakeSource{}).Run(&out, false, false, false))

		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, "no sessions\n", out.String())
		})
	})

	t.Run("when the source fails", func(t *testing.T) {
		err := commands.NewStatus(&fakeSource{err: errors.New("boom")}).Run(&bytes.Buffer{}, false, false, false)

		t.Run("it should return the error", func(t *testing.T) {
			assert.ErrorContains(t, err, "boom")
		})
	})
}

func TestFormatting(t *testing.T) {
	t.Run("when formatting token rates", func(t *testing.T) {
		for _, c := range []struct {
			in   float64
			want string
		}{{0, "-"}, {999, "999"}, {1000, "1k"}, {152_000, "152k"}, {8_900_000, "8.9M"}} {
			t.Run("it should render "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, format.Tokens(c.in))
			})
		}
	})

	t.Run("when formatting ages", func(t *testing.T) {
		for _, c := range []struct {
			in   time.Duration
			want string
		}{{-time.Second, "-"}, {6 * time.Minute, "6m"}, {5*time.Hour + 6*time.Minute, "5h06m"}, {49 * time.Hour, "2d1h"}} {
			t.Run("it should render "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, format.Age(c.in))
			})
		}
	})

	t.Run("when formatting percentages", func(t *testing.T) {
		for _, c := range []struct {
			in   float64
			want string
		}{{0, "-"}, {25.4, "25%"}, {99.6, "100%"}} {
			t.Run("it should render "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, format.Percent(c.in))
			})
		}
	})
}

func TestDaemonOrDirect(t *testing.T) {
	t.Run("when no daemon answers on the socket", func(t *testing.T) {
		home := helpers.NewHome(t).Session(4242, sid, cwd, "interactive", "idle")
		var notice bytes.Buffer
		source := commands.DaemonOrDirect{
			Home: home.Path, Socket: home.Path + "/none.sock",
			Probes: state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }},
			Now:    time.Now, Notice: &notice,
		}
		snapshot, err := source.Snapshot(false)
		require.NoError(t, err)

		t.Run("it should scan directly", func(t *testing.T) {
			assert.Len(t, snapshot.Sessions, 1)
		})

		t.Run("it should say so on the notice writer", func(t *testing.T) {
			assert.Contains(t, notice.String(), "not reachable")
		})
	})

	t.Run("when asked for a direct scan", func(t *testing.T) {
		home := helpers.NewHome(t)
		var notice bytes.Buffer
		source := commands.DaemonOrDirect{Home: home.Path, Socket: "/nowhere.sock",
			Probes: state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }},
			Now:    time.Now, Notice: &notice}
		_, err := source.Snapshot(true)
		require.NoError(t, err)

		t.Run("it should not mention the daemon at all", func(t *testing.T) {
			assert.Empty(t, notice.String())
		})
	})
}

func TestSessions(t *testing.T) {
	t.Run("when a session is started in a relative directory", func(t *testing.T) {
		ctl := &fakeController{id: "0898d7e4"}
		id, err := commands.NewSessions(t.TempDir(), ctl).New(context.Background(), ".", "fix it")
		require.NoError(t, err)

		t.Run("it should resolve the directory to an absolute path", func(t *testing.T) {
			assert.True(t, strings.HasPrefix(ctl.calls[0], "new /"), ctl.calls[0])
		})

		t.Run("it should return the job id", func(t *testing.T) {
			assert.Equal(t, "0898d7e4", id)
		})
	})

	t.Run("when a session is resumed without a directory", func(t *testing.T) {
		home := helpers.NewHome(t)
		home.Transcript(sid, cwd, helpers.UserPrompt(sid, cwd, "2026-09-16T19:50:00.000Z"))
		ctl := &fakeController{id: "08b1d655"}
		_, err := commands.NewSessions(home.Path, ctl).Resume(context.Background(), "", sid)
		require.NoError(t, err)

		t.Run("it should resume in the transcript's cwd and then attach", func(t *testing.T) {
			assert.Equal(t, []string{"resume " + cwd + " " + sid, "attach 08b1d655"}, ctl.calls)
		})
	})

	t.Run("when a session is resumed with an explicit directory", func(t *testing.T) {
		ctl := &fakeController{id: "08b1d655"}
		_, err := commands.NewSessions(t.TempDir(), ctl).Resume(context.Background(), "/tmp/elsewhere", sid)
		require.NoError(t, err)

		t.Run("it should resume there", func(t *testing.T) {
			assert.Equal(t, "resume /tmp/elsewhere "+sid, ctl.calls[0])
		})
	})

	t.Run("when a resumed session has no transcript", func(t *testing.T) {
		ctl := &fakeController{}
		_, err := commands.NewSessions(t.TempDir(), ctl).Resume(context.Background(), "", sid)

		t.Run("it should ask for --dir", func(t *testing.T) {
			assert.ErrorContains(t, err, "--dir")
		})

		t.Run("it should call nothing", func(t *testing.T) {
			assert.Empty(t, ctl.calls)
		})
	})
}

func TestHooks(t *testing.T) {
	t.Run("when hooks are installed then removed", func(t *testing.T) {
		home := t.TempDir()
		h := commands.NewHooks(home, "botropolis-hook")
		var out bytes.Buffer
		require.NoError(t, h.Install(&out))

		t.Run("it should report the install", func(t *testing.T) {
			assert.Contains(t, out.String(), "installed for 9 events")
		})

		t.Run("and installed again", func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, h.Install(&out))

			t.Run("it should report nothing to change", func(t *testing.T) {
				assert.Contains(t, out.String(), "nothing to change")
			})
		})

		t.Run("and removed", func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, h.Remove(&out))

			t.Run("it should report the removal", func(t *testing.T) {
				assert.Contains(t, out.String(), "removed")
			})
		})
	})

	t.Run("when a dry run is requested", func(t *testing.T) {
		var out bytes.Buffer
		commands.NewHooks(t.TempDir(), "/usr/bin/botropolis-hook").DryRun(&out)

		t.Run("it should print the block with the command", func(t *testing.T) {
			assert.Contains(t, out.String(), `"/usr/bin/botropolis-hook"`)
		})
	})
}

func TestPrune(t *testing.T) {
	now := time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)
	agents := []control.Agent{
		{ID: "11111111", SessionID: "11111111-0000-0000-0000-000000000000", Kind: "background", StartedAt: now.Add(-10 * 24 * time.Hour).UnixMilli()},
		{ID: "22222222", SessionID: "22222222-0000-0000-0000-000000000000", Kind: "background", StartedAt: now.Add(-time.Hour).UnixMilli()},
		{ID: "33333333", SessionID: "33333333-0000-0000-0000-000000000000", Kind: "background", StartedAt: now.Add(-30 * 24 * time.Hour).UnixMilli()},
		{SessionID: "44444444-0000-0000-0000-000000000000", Kind: "interactive", StartedAt: now.Add(-30 * 24 * time.Hour).UnixMilli()},
	}
	snapshot := state.Snapshot{At: now, Sessions: []state.Session{
		{ID: "11111111-0000-0000-0000-000000000000", State: state.Resting, Title: "old job", LastActivity: now.Add(-9 * 24 * time.Hour)},
		{ID: "33333333-0000-0000-0000-000000000000", State: state.Working, Title: "still running", LastActivity: now.Add(-time.Minute)},
	}}

	t.Run("when pruning parked background jobs older than a week", func(t *testing.T) {
		ctl := &fakeController{agents: agents}
		pruned, err := commands.NewSessions(t.TempDir(), ctl).Prune(context.Background(), snapshot, 7*24*time.Hour, now, false)
		require.NoError(t, err)

		t.Run("it should remove only the parked job that is old enough", func(t *testing.T) {
			assert.Equal(t, []string{"agents all=true", "rm 11111111"}, ctl.calls)
		})

		t.Run("it should report what it removed with the title from the snapshot", func(t *testing.T) {
			require.Len(t, pruned, 1)
			assert.Equal(t, commands.Pruned{ID: "11111111", Title: "old job", LastSeen: now.Add(-9 * 24 * time.Hour)}, pruned[0])
		})
	})

	t.Run("when the run is a dry run", func(t *testing.T) {
		ctl := &fakeController{agents: agents}
		pruned, err := commands.NewSessions(t.TempDir(), ctl).Prune(context.Background(), snapshot, 7*24*time.Hour, now, true)
		require.NoError(t, err)

		t.Run("it should report but not remove", func(t *testing.T) {
			assert.Len(t, pruned, 1)
			assert.Equal(t, []string{"agents all=true"}, ctl.calls)
		})
	})

	t.Run("when an empty job has sat for over an hour", func(t *testing.T) {
		ctl := &fakeController{agents: []control.Agent{{ID: "66666666", SessionID: "66666666-0000-0000-0000-000000000000", Kind: "background", StartedAt: now.Add(-2 * time.Hour).UnixMilli()}}}
		blank := state.Snapshot{Sessions: []state.Session{{ID: "66666666-0000-0000-0000-000000000000", State: state.Empty, LastActivity: now.Add(-2 * time.Hour)}}}
		pruned, err := commands.NewSessions(t.TempDir(), ctl).Prune(context.Background(), blank, 7*24*time.Hour, now, false)
		require.NoError(t, err)

		t.Run("it should remove it inside the parked cutoff", func(t *testing.T) {
			assert.Equal(t, []string{"agents all=true", "rm 66666666"}, ctl.calls)
		})

		t.Run("it should name it by its session id", func(t *testing.T) {
			require.Len(t, pruned, 1)
			assert.Equal(t, "66666666-0000-0000-0000-000000000000", pruned[0].Title)
		})
	})

	t.Run("when an empty job is younger than an hour", func(t *testing.T) {
		ctl := &fakeController{agents: []control.Agent{{ID: "77777777", SessionID: "77777777-0000-0000-0000-000000000000", Kind: "background", StartedAt: now.Add(-30 * time.Minute).UnixMilli()}}}
		blank := state.Snapshot{Sessions: []state.Session{{ID: "77777777-0000-0000-0000-000000000000", State: state.Empty, LastActivity: now.Add(-30 * time.Minute)}}}
		pruned, err := commands.NewSessions(t.TempDir(), ctl).Prune(context.Background(), blank, 7*24*time.Hour, now, false)
		require.NoError(t, err)

		t.Run("it should leave it alone", func(t *testing.T) {
			assert.Empty(t, pruned)
		})
	})

	t.Run("when a job is unknown to the snapshot", func(t *testing.T) {
		ctl := &fakeController{agents: []control.Agent{{ID: "55555555", SessionID: "55555555-0000-0000-0000-000000000000", Kind: "background", StartedAt: now.Add(-40 * 24 * time.Hour).UnixMilli()}}}
		pruned, err := commands.NewSessions(t.TempDir(), ctl).Prune(context.Background(), state.Snapshot{}, 7*24*time.Hour, now, false)
		require.NoError(t, err)

		t.Run("it should fall back to the job's start time", func(t *testing.T) {
			require.Len(t, pruned, 1)
			assert.Equal(t, "55555555-0000-0000-0000-000000000000", pruned[0].Title)
		})
	})

	t.Run("when listing fails", func(t *testing.T) {
		ctl := &fakeController{failure: errors.New("boom")}
		_, err := commands.NewSessions(t.TempDir(), ctl).Prune(context.Background(), snapshot, time.Hour, now, false)

		t.Run("it should return the error", func(t *testing.T) {
			assert.ErrorContains(t, err, "boom")
		})
	})
}

func TestActorDemolish(t *testing.T) {
	t.Run("when a building is demolished", func(t *testing.T) {
		ctl := &fakeController{}
		require.NoError(t, commands.Actor{Sessions: commands.NewSessions(t.TempDir(), ctl)}.Do(city.Action{Kind: city.ActionDemolish, SessionID: "0898d7e4"}))

		t.Run("it should remove the session", func(t *testing.T) {
			assert.Equal(t, []string{"rm 0898d7e4"}, ctl.calls)
		})
	})
}
