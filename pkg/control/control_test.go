package control_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type call struct {
	dir  string
	argv []string
}

type fakeRunner struct {
	runs    []call
	starts  []call
	feeds   []feed
	here    []call
	stdout  string
	agents  string
	failure error

	feedDeadline time.Time
	// hang makes Run wait out its context, as a claude that never
	// answers would, killed at the deadline.
	hang      bool
	deadlines []time.Time

	reads        []call
	readDeadline time.Time
	readHangs    bool
}

func (f *fakeRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	f.runs = append(f.runs, call{dir, append([]string{name}, args...)})
	deadline, _ := ctx.Deadline()
	f.deadlines = append(f.deadlines, deadline)
	if f.hang {
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		return "", errors.New("signal: killed")
	}
	if len(args) > 0 && args[0] == "agents" {
		return f.agents, f.failure
	}
	return f.stdout, f.failure
}

func (f *fakeRunner) Start(dir string, name string, args ...string) error {
	f.starts = append(f.starts, call{dir, append([]string{name}, args...)})
	return f.failure
}

func (f *fakeRunner) Interactive(name string, args ...string) error {
	f.here = append(f.here, call{"", append([]string{name}, args...)})
	return f.failure
}

func (f *fakeRunner) Feed(ctx context.Context, input string, name string, args ...string) error {
	f.feedDeadline, _ = ctx.Deadline()
	f.feeds = append(f.feeds, feed{input, append([]string{name}, args...)})
	return f.failure
}

func (f *fakeRunner) Read(ctx context.Context, name string, args ...string) (string, error) {
	f.readDeadline, _ = ctx.Deadline()
	f.reads = append(f.reads, call{"", append([]string{name}, args...)})
	if f.readHangs {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return f.stdout, f.failure
}

type feed struct {
	input string
	argv  []string
}

const (
	sessionID = "0898d7e4-f2cb-45fe-833e-1921c1f4bcaa"
	jobID     = "0898d7e4"
	dir       = "/home/avesta/workspaces/github/auroq/botropolis"
)

func newControl(runner *fakeRunner) *control.Control {
	return control.New(runner, linux, env(map[string]string{"TERMINAL": "kitty"}), onPath())
}

const backgrounded = "\x1b[2mbackgrounded \xc2\xb7 \x1b[36m16daefa1\x1b[39m \xc2\xb7 Pong\x1b[22m\n" +
	"  claude agents              list sessions\n  claude attach 16daefa1     open in this terminal\n"

const agentsJSON = `[{"pid":16548,"cwd":"/elsewhere","kind":"interactive","startedAt":1,"sessionId":"1377da2f-0000-0000-0000-000000000000"},` +
	`{"pid":1,"id":"aaaaaaaa","cwd":"` + dir + `","kind":"background","startedAt":1789617000000,"sessionId":"aaaaaaaa-0000-0000-0000-000000000000"},` +
	`{"pid":2,"id":"bbbbbbbb","cwd":"` + dir + `","kind":"background","startedAt":1789618000000,"sessionId":"bbbbbbbb-0000-0000-0000-000000000000"}]`

func TestControl(t *testing.T) {
	t.Run("when a new session is started in a directory with a prompt", func(t *testing.T) {
		runner := &fakeRunner{stdout: backgrounded}
		id, err := newControl(runner).New(context.Background(), dir, "fix the CI queue")
		require.NoError(t, err)
		require.Len(t, runner.runs, 1)

		t.Run("it should run claude --bg with the prompt in that directory", func(t *testing.T) {
			assert.Equal(t, call{dir, []string{"claude", "--bg", "fix the CI queue"}}, runner.runs[0])
		})

		t.Run("it should read the job id claude printed", func(t *testing.T) {
			assert.Equal(t, "16daefa1", id)
		})
	})

	t.Run("when a new session is started without a prompt", func(t *testing.T) {
		runner := &fakeRunner{stdout: backgrounded}
		_, err := newControl(runner).New(context.Background(), dir, "")
		require.NoError(t, err)

		t.Run("it should not pass an empty argument", func(t *testing.T) {
			assert.Equal(t, []string{"claude", "--bg"}, runner.runs[0].argv)
		})
	})

	t.Run("when claude starts a session but prints nothing recognisable", func(t *testing.T) {
		runner := &fakeRunner{stdout: "something new\n", agents: agentsJSON}
		id, err := newControl(runner).New(context.Background(), dir, "")
		require.NoError(t, err)

		t.Run("it should ask claude agents and take the newest background job in that directory", func(t *testing.T) {
			assert.Equal(t, "bbbbbbbb", id)
		})
	})

	t.Run("when claude prints nothing recognisable and lists no job in that directory", func(t *testing.T) {
		runner := &fakeRunner{stdout: "", agents: `[]`}
		_, err := newControl(runner).New(context.Background(), dir, "")

		t.Run("it should return an error", func(t *testing.T) {
			assert.ErrorContains(t, err, "job id")
		})
	})

	t.Run("when claude fails to start a session", func(t *testing.T) {
		runner := &fakeRunner{failure: errors.New("exit status 1: no such directory")}
		_, err := newControl(runner).New(context.Background(), dir, "")

		t.Run("it should return the failure", func(t *testing.T) {
			assert.ErrorContains(t, err, "no such directory")
		})
	})

	t.Run("when claude refuses a directory nobody has trusted", func(t *testing.T) {
		untrusted := errors.New("exit status 1: Workspace not trusted. Run `claude` in " + dir + " once and accept the trust prompt, then retry.")

		for _, start := range []struct {
			name string
			call func(*control.Control) error
		}{
			{"and a new session is started", func(c *control.Control) error {
				_, err := c.New(context.Background(), dir, "")
				return err
			}},
			{"and a resting session is resumed", func(c *control.Control) error {
				_, err := c.Resume(context.Background(), dir, sessionID)
				return err
			}},
		} {
			t.Run(start.name, func(t *testing.T) {
				err := start.call(newControl(&fakeRunner{failure: untrusted}))

				t.Run("it should report the directory as untrusted", func(t *testing.T) {
					assert.ErrorIs(t, err, control.ErrUntrusted)
				})

				t.Run("it should keep claude's own explanation", func(t *testing.T) {
					assert.ErrorContains(t, err, "accept the trust prompt")
				})
			})
		}
	})

	t.Run("when claude fails for any other reason", func(t *testing.T) {
		_, err := newControl(&fakeRunner{failure: errors.New("exit status 1: no such directory")}).New(context.Background(), dir, "")

		t.Run("it should not report the directory as untrusted", func(t *testing.T) {
			assert.NotErrorIs(t, err, control.ErrUntrusted)
		})
	})

	t.Run("when a session is attached", func(t *testing.T) {
		runner := &fakeRunner{}
		err := newControl(runner).Attach(jobID, dir)
		require.NoError(t, err)
		require.Len(t, runner.starts, 1)

		t.Run("it should spawn a terminal running only claude attach, with nothing after it", func(t *testing.T) {
			assert.Equal(t, []string{"kitty", "claude", "attach", jobID}, runner.starts[0].argv)
		})

		t.Run("it should start the terminal in the session's directory", func(t *testing.T) {
			assert.Equal(t, dir, runner.starts[0].dir)
		})

		t.Run("it should not block on the terminal", func(t *testing.T) {
			assert.Empty(t, runner.runs)
		})
	})

	t.Run("when a session is attached with no directory known", func(t *testing.T) {
		runner := &fakeRunner{}

		t.Run("it should still spawn the terminal", func(t *testing.T) {
			require.NoError(t, newControl(runner).Attach(jobID, ""))
			assert.Len(t, runner.starts, 1)
		})
	})

	t.Run("when a session is attached in this terminal", func(t *testing.T) {
		runner := &fakeRunner{}
		require.NoError(t, newControl(runner).AttachHere(sessionID))

		t.Run("it should run claude attach on the caller's own terminal", func(t *testing.T) {
			assert.Equal(t, []call{{"", []string{"claude", "attach", jobID}}}, runner.here)
		})

		t.Run("it should open no other terminal", func(t *testing.T) {
			assert.Empty(t, runner.starts)
		})
	})

	t.Run("when a session is stopped", func(t *testing.T) {
		runner := &fakeRunner{}
		require.NoError(t, newControl(runner).Stop(context.Background(), jobID))

		t.Run("it should run claude stop", func(t *testing.T) {
			assert.Equal(t, []string{"claude", "stop", jobID}, runner.runs[0].argv)
		})
	})

	t.Run("when a session is removed", func(t *testing.T) {
		runner := &fakeRunner{}
		require.NoError(t, newControl(runner).Remove(context.Background(), jobID))

		t.Run("it should run claude rm", func(t *testing.T) {
			assert.Equal(t, []string{"claude", "rm", jobID}, runner.runs[0].argv)
		})
	})

	t.Run("when a session is forked", func(t *testing.T) {
		runner := &fakeRunner{stdout: backgrounded}
		_, err := newControl(runner).Fork(context.Background(), dir, sessionID)
		require.NoError(t, err)

		t.Run("it should run claude --bg --resume --fork-session in the session's directory", func(t *testing.T) {
			assert.Equal(t, call{dir, []string{"claude", "--bg", "--resume", sessionID, "--fork-session"}}, runner.runs[0])
		})
	})

	t.Run("when a resting session is resumed", func(t *testing.T) {
		runner := &fakeRunner{stdout: backgrounded}
		id, err := newControl(runner).Resume(context.Background(), dir, sessionID)
		require.NoError(t, err)

		t.Run("it should run claude --bg --resume in the session's directory", func(t *testing.T) {
			assert.Equal(t, call{dir, []string{"claude", "--bg", "--resume", sessionID}}, runner.runs[0])
		})

		t.Run("it should read the job id claude printed", func(t *testing.T) {
			assert.Equal(t, "16daefa1", id)
		})
	})

	t.Run("when a job id is not eight hex characters", func(t *testing.T) {
		runner := &fakeRunner{}
		err := newControl(runner).Stop(context.Background(), "../../etc")

		t.Run("it should refuse before running anything", func(t *testing.T) {
			assert.Error(t, err)
			assert.Empty(t, runner.runs)
		})
	})

	t.Run("when a job id is a full session id", func(t *testing.T) {
		runner := &fakeRunner{}
		require.NoError(t, newControl(runner).Stop(context.Background(), sessionID))

		t.Run("it should shorten it to the job id", func(t *testing.T) {
			assert.True(t, strings.HasSuffix(strings.Join(runner.runs[0].argv, " "), " "+jobID))
		})
	})
}

func TestAgents(t *testing.T) {
	t.Run("when all agents are listed", func(t *testing.T) {
		runner := &fakeRunner{agents: agentsJSON}
		agents, err := newControl(runner).Agents(context.Background(), true)
		require.NoError(t, err)

		t.Run("it should ask claude for everything", func(t *testing.T) {
			assert.Equal(t, []string{"claude", "agents", "--all", "--json"}, runner.runs[0].argv)
		})

		t.Run("it should return every entry", func(t *testing.T) {
			assert.Len(t, agents, 3)
		})

		t.Run("it should read the job id and kind", func(t *testing.T) {
			assert.Equal(t, "aaaaaaaa", agents[1].ID)
			assert.Equal(t, "background", agents[1].Kind)
		})
	})

	t.Run("when only running agents are listed", func(t *testing.T) {
		runner := &fakeRunner{agents: `[]`}
		_, err := newControl(runner).Agents(context.Background(), false)
		require.NoError(t, err)

		t.Run("it should not pass --all", func(t *testing.T) {
			assert.Equal(t, []string{"claude", "agents", "--json"}, runner.runs[0].argv)
		})
	})

	t.Run("when the listing is not JSON", func(t *testing.T) {
		_, err := newControl(&fakeRunner{agents: "nope"}).Agents(context.Background(), true)

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func TestRevealAndCopy(t *testing.T) {
	t.Run("when nothing is given to reveal", func(t *testing.T) {
		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, newControl(&fakeRunner{}).Reveal(""))
		})
	})

	t.Run("when no clipboard tool is on the path", func(t *testing.T) {
		c := control.New(&fakeRunner{}, linux, env(nil), func(string) bool { return false })

		t.Run("it should say which to install", func(t *testing.T) {
			assert.ErrorContains(t, c.Copy(context.Background(), dir), "clipboard")
		})
	})

	t.Run("when a clipboard tool is on the path", func(t *testing.T) {
		runner := &fakeRunner{}
		copied := time.Now()
		require.NoError(t, control.New(runner, linux, env(nil), onPath("xclip")).Copy(context.Background(), dir))

		t.Run("it should hand the tool a deadline no more than two seconds away", func(t *testing.T) {
			assert.WithinDuration(t, copied.Add(control.CopyTimeout), runner.feedDeadline, 100*time.Millisecond)
		})
	})
}
