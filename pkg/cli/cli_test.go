package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/cli"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
)

type fakeStatus struct {
	direct   bool
	all      bool
	asJSON   bool
	err      error
	snapshot state.Snapshot
}

func (f *fakeStatus) Run(out io.Writer, direct, all, asJSON bool) error {
	f.direct, f.all, f.asJSON = direct, all, asJSON
	_, _ = io.WriteString(out, "TABLE\n")
	return f.err
}

func (f *fakeStatus) Snapshot(direct bool) (state.Snapshot, error) {
	f.direct = direct
	return f.snapshot, f.err
}

type fakeHooks struct{ calls []string }

func (f *fakeHooks) Install(io.Writer) error { f.calls = append(f.calls, "install"); return nil }
func (f *fakeHooks) Remove(io.Writer) error  { f.calls = append(f.calls, "remove"); return nil }
func (f *fakeHooks) DryRun(io.Writer)        { f.calls = append(f.calls, "dry-run") }

type fakeSessions struct {
	calls []string
	id    string
	err   error
}

func (f *fakeSessions) New(_ context.Context, dir, prompt string) (string, error) {
	f.calls = append(f.calls, "new "+dir+" "+prompt)
	return f.id, f.err
}
func (f *fakeSessions) Attach(id string) error { f.calls = append(f.calls, "attach "+id); return f.err }
func (f *fakeSessions) Stop(_ context.Context, id string) error {
	f.calls = append(f.calls, "stop "+id)
	return f.err
}
func (f *fakeSessions) Remove(_ context.Context, id string) error {
	f.calls = append(f.calls, "rm "+id)
	return f.err
}
func (f *fakeSessions) Resume(_ context.Context, dir, sessionID string) (string, error) {
	f.calls = append(f.calls, "resume "+dir+" "+sessionID)
	return f.id, f.err
}

func (f *fakeSessions) Prune(_ context.Context, _ state.Snapshot, olderThan time.Duration, _ time.Time, dryRun bool) ([]commands.Pruned, error) {
	f.calls = append(f.calls, fmt.Sprintf("prune %s dry=%v", olderThan, dryRun))
	if f.err != nil {
		return nil, f.err
	}
	return []commands.Pruned{{ID: "11111111", Title: "old job"}}, nil
}

type fakeBar struct {
	calls []string
}

func (f *fakeBar) Once(out io.Writer, format commands.BarFormat, direct bool) error {
	f.calls = append(f.calls, fmt.Sprintf("once %s direct=%v", format, direct))
	_, _ = io.WriteString(out, "LINE\n")
	return nil
}

func (f *fakeBar) Watch(_ context.Context, out io.Writer, format commands.BarFormat) {
	f.calls = append(f.calls, fmt.Sprintf("watch %s", format))
	_, _ = io.WriteString(out, "LINE\nLINE\n")
}

type fakeNotify struct{ runs int }

func (f *fakeNotify) Run(context.Context) { f.runs++ }

type fakeDoctor struct{ runs int }

func (f *fakeDoctor) Report(out io.Writer) error {
	f.runs++
	_, err := io.WriteString(out, "all good\n")
	return err
}

func (h *harness) Doctor(*config.Config) cli.DoctorRunner { return h.doctor }

type fakeTUI struct{ runs int }

func (f *fakeTUI) Run(context.Context) error { f.runs++; return nil }

type fakeCity struct {
	runs       int
	screenshot string
	keys       []string
	record     string
	seconds    float64
	headless   bool
}

func (f *fakeCity) Run(cmd *cobra.Command) error {
	f.runs++
	f.screenshot = cli.Screenshot(cmd)
	f.keys = cli.Keys(cmd)
	f.record, f.seconds = cli.Record(cmd)
	f.headless = cli.Headless(cmd)
	return nil
}

type fakeEvents struct {
	since time.Duration
	runs  int
}

func (f *fakeEvents) Run(out io.Writer, since time.Time) error {
	f.runs++
	f.since = time.Since(since).Round(time.Hour)
	_, _ = io.WriteString(out, "EVENTS\n")
	return nil
}

type harness struct {
	events   *fakeEvents
	status   *fakeStatus
	hooks    *fakeHooks
	sessions *fakeSessions
	city     *fakeCity
	bar      *fakeBar
	notify   *fakeNotify
	tui      *fakeTUI
	doctor   *fakeDoctor
	find     *fakeFind
	seen     *config.Config
	out      bytes.Buffer
	root     *cobra.Command
}

func (h *harness) Status(cfg *config.Config) cli.StatusRunner     { h.seen = cfg; return h.status }
func (h *harness) Hooks(cfg *config.Config) cli.HooksRunner       { h.seen = cfg; return h.hooks }
func (h *harness) Sessions(cfg *config.Config) cli.SessionsRunner { h.seen = cfg; return h.sessions }
func (h *harness) City(cfg *config.Config) cli.CityRunner         { h.seen = cfg; return h.city }
func (h *harness) Bar(cfg *config.Config) cli.BarRunner           { h.seen = cfg; return h.bar }
func (h *harness) Notify(cfg *config.Config) cli.NotifyRunner     { h.seen = cfg; return h.notify }
func (h *harness) TUI(cfg *config.Config) cli.TUIRunner           { h.seen = cfg; return h.tui }
func (h *harness) Events(cfg *config.Config) cli.EventsRunner     { h.seen = cfg; return h.events }
func (h *harness) Find(cfg *config.Config) cli.FindRunner         { h.seen = cfg; return h.find }

func newHarness(cfg *config.Config) *harness {
	h := &harness{status: &fakeStatus{}, hooks: &fakeHooks{}, sessions: &fakeSessions{id: "0898d7e4"}, city: &fakeCity{}, bar: &fakeBar{}, notify: &fakeNotify{}, tui: &fakeTUI{}, doctor: &fakeDoctor{}, events: &fakeEvents{}, find: &fakeFind{}}
	load := func() (*config.Config, error) { return cfg, nil }
	v := config.NewViper()
	subs := append([]*cobra.Command{cli.NewStatusCLI(load, h), cli.NewInstallHooksCLI(load, h), cli.NewCityCLI(load, h), cli.NewBarCLI(load, h), cli.NewNotifyCLI(load, h), cli.NewTUICLI(load, h), cli.NewDoctorCLI(load, h), cli.NewEventsCLI(load, h), cli.NewFindCLI(load, h)},
		cli.NewSessionCLIs(load, h)...)
	h.root = cli.NewRootCLI(v, subs...)
	h.root.SetOut(&h.out)
	h.root.SetErr(io.Discard)
	return h
}

func (h *harness) run(args ...string) error {
	h.root.SetArgs(args)
	return h.root.Execute()
}

func TestEventsCLI(t *testing.T) {
	t.Run("when events is run with a window", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("events", "--since", "2h"))

		t.Run("it should ask for the daemon's log over that window", func(t *testing.T) {
			assert.Equal(t, 1, h.events.runs)
			assert.Equal(t, 2*time.Hour, h.events.since)
			assert.Contains(t, h.out.String(), "EVENTS")
		})
	})

	t.Run("when events is run without a window", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("events"))

		t.Run("it should default to a day", func(t *testing.T) {
			assert.Equal(t, 24*time.Hour, h.events.since)
		})
	})
}

func TestDoctorCLI(t *testing.T) {
	t.Run("when doctor is run", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("doctor"))

		t.Run("it should report", func(t *testing.T) {
			assert.Equal(t, 1, h.doctor.runs)
			assert.Contains(t, h.out.String(), "all good")
		})
	})
}

func TestRootCLI(t *testing.T) {
	t.Run("when asked for the version", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("--version"))

		t.Run("it should print the binary name and version", func(t *testing.T) {
			assert.Equal(t, "botropolis dev\n", h.out.String())
		})
	})

	t.Run("when run with the city subcommand", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("city"))

		t.Run("it should open the city", func(t *testing.T) {
			assert.Equal(t, 1, h.city.runs)
		})
	})

	t.Run("when run with no arguments at all", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run())

		t.Run("it should open the city too", func(t *testing.T) {
			assert.Equal(t, 1, h.city.runs)
		})
	})

	t.Run("when asked for a screenshot", func(t *testing.T) {
		t.Run("and the city subcommand is used", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--screenshot", "out.png"))

			t.Run("it should hand the path to the city", func(t *testing.T) {
				assert.Equal(t, "out.png", h.city.screenshot)
			})
		})

		t.Run("and no subcommand is used", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("--screenshot", "out.png"))

			t.Run("it should hand the path to the city as well", func(t *testing.T) {
				assert.Equal(t, "out.png", h.city.screenshot)
			})
		})

		t.Run("and no path is given", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city"))

			t.Run("it should ask for no screenshot", func(t *testing.T) {
				assert.Empty(t, h.city.screenshot)
			})
		})

		t.Run("and a recording is asked for", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--record", "frames", "--seconds", "12"))

			t.Run("it should hand the directory and the length to the city", func(t *testing.T) {
				assert.Equal(t, "frames", h.city.record)
				assert.Equal(t, 12.0, h.city.seconds)
			})
		})

		t.Run("and headless is asked for", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--headless", "--screenshot", "out.png"))

			t.Run("it should tell the city", func(t *testing.T) {
				assert.True(t, h.city.headless)
			})
		})

		t.Run("and keys are given", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run("city", "--screenshot", "out.png", "--keys", "equal,equal,b"))

			t.Run("it should hand the keys to the city in order", func(t *testing.T) {
				assert.Equal(t, []string{"equal", "equal", "b"}, h.city.keys)
			})
		})
	})

	t.Run("when run with --tui", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("--tui"))

		t.Run("it should open the terminal table instead of the city", func(t *testing.T) {
			assert.Equal(t, 1, h.tui.runs)
			assert.Equal(t, 0, h.city.runs)
		})
	})

	t.Run("when run with the tui subcommand", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("tui"))

		t.Run("it should open the terminal table", func(t *testing.T) {
			assert.Equal(t, 1, h.tui.runs)
		})
	})

	t.Run("when given an unknown command", func(t *testing.T) {
		err := newHarness(&config.Config{}).run("dance")

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func TestStatusCLI(t *testing.T) {
	t.Run("when run with neither home nor socket set", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("status"))

		t.Run("it should ask the daemon first", func(t *testing.T) {
			assert.False(t, h.status.direct)
		})

		t.Run("it should print the table", func(t *testing.T) {
			assert.Equal(t, "TABLE\n", h.out.String())
		})

		t.Run("it should build the service from the loaded config", func(t *testing.T) {
			assert.NotNil(t, h.seen)
		})
	})

	t.Run("when run with --all", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("status", "--all"))

		t.Run("it should ask for resting sessions too", func(t *testing.T) {
			assert.True(t, h.status.all)
		})
	})

	t.Run("when run with --direct", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("status", "--direct"))

		t.Run("it should scan directly", func(t *testing.T) {
			assert.True(t, h.status.direct)
		})
	})

	t.Run("when home was set explicitly but the socket was not", func(t *testing.T) {
		h := newHarness(&config.Config{HomeSet: true})
		require.NoError(t, h.run("status"))

		t.Run("it should scan directly", func(t *testing.T) {
			assert.True(t, h.status.direct)
		})
	})

	t.Run("when both home and socket were set explicitly", func(t *testing.T) {
		h := newHarness(&config.Config{HomeSet: true, SocketSet: true})
		require.NoError(t, h.run("status"))

		t.Run("it should ask the daemon", func(t *testing.T) {
			assert.False(t, h.status.direct)
		})
	})

	t.Run("when the status runner fails", func(t *testing.T) {
		h := newHarness(&config.Config{})
		h.status.err = errors.New("boom")

		t.Run("it should return the error", func(t *testing.T) {
			assert.ErrorContains(t, h.run("status"), "boom")
		})
	})
}

func TestInstallHooksCLI(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"install-hooks"}, "install"},
		{[]string{"install-hooks", "--remove"}, "remove"},
		{[]string{"install-hooks", "--dry-run"}, "dry-run"},
		{[]string{"install-hooks", "--dry-run", "--remove"}, "dry-run"},
	} {
		t.Run("when run as "+c.want, func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run(c.args...))

			t.Run("it should call "+c.want, func(t *testing.T) {
				assert.Equal(t, []string{c.want}, h.hooks.calls)
			})
		})
	}
}

func TestSessionCLIs(t *testing.T) {
	t.Run("when new is given a directory and a prompt", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("new", "/tmp/project", "fix", "the", "queue"))

		t.Run("it should start the session with the joined prompt", func(t *testing.T) {
			assert.Equal(t, []string{"new /tmp/project fix the queue"}, h.sessions.calls)
		})

		t.Run("it should print only the job id", func(t *testing.T) {
			assert.Equal(t, "0898d7e4\n", h.out.String())
		})
	})

	t.Run("when new is given no directory", func(t *testing.T) {
		h := newHarness(&config.Config{})

		t.Run("it should fail before calling anything", func(t *testing.T) {
			assert.Error(t, h.run("new"))
			assert.Empty(t, h.sessions.calls)
		})
	})

	for _, verb := range []string{"attach", "stop", "rm"} {
		t.Run("when "+verb+" is given an id", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run(verb, "0898d7e4"))

			t.Run("it should hand the id on", func(t *testing.T) {
				assert.Equal(t, []string{verb + " 0898d7e4"}, h.sessions.calls)
			})
		})

		t.Run("when "+verb+" is given no id", func(t *testing.T) {
			h := newHarness(&config.Config{})

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, h.run(verb))
			})
		})
	}

	t.Run("when prune is run with a cutoff and dry-run", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("prune", "--older-than", "48h", "--dry-run"))

		t.Run("it should take a direct snapshot first", func(t *testing.T) {
			assert.True(t, h.status.direct)
		})

		t.Run("it should hand the cutoff and dry-run on", func(t *testing.T) {
			assert.Equal(t, []string{"prune 48h0m0s dry=true"}, h.sessions.calls)
		})

		t.Run("it should say what it would remove", func(t *testing.T) {
			assert.Contains(t, h.out.String(), "would remove 11111111  old job")
		})
	})

	t.Run("when resume is given a session id and a directory", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("resume", "--dir", "/tmp/elsewhere", "08b1d655-276f-495b-a61f-2214af35a97a"))

		t.Run("it should pass both on", func(t *testing.T) {
			assert.Equal(t, []string{"resume /tmp/elsewhere 08b1d655-276f-495b-a61f-2214af35a97a"}, h.sessions.calls)
		})

		t.Run("it should print the job id", func(t *testing.T) {
			assert.Equal(t, "0898d7e4\n", h.out.String())
		})
	})

	t.Run("when the sessions runner fails", func(t *testing.T) {
		h := newHarness(&config.Config{})
		h.sessions.err = errors.New("claude: boom")

		t.Run("it should return the error", func(t *testing.T) {
			assert.ErrorContains(t, h.run("stop", "0898d7e4"), "boom")
		})
	})
}

func TestBarCLI(t *testing.T) {
	t.Run("when run with defaults", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("bar"))

		t.Run("it should print one waybar line via the daemon", func(t *testing.T) {
			assert.Equal(t, []string{"once waybar direct=false"}, h.bar.calls)
			assert.Equal(t, "LINE\n", h.out.String())
		})
	})

	t.Run("when run with --format text --watch", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("bar", "--format", "text", "--watch"))

		t.Run("it should stream text lines", func(t *testing.T) {
			assert.Equal(t, []string{"watch text"}, h.bar.calls)
		})
	})

	t.Run("when run with an unknown format", func(t *testing.T) {
		h := newHarness(&config.Config{})

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, h.run("bar", "--format", "xml"))
		})
	})

	t.Run("when home is set explicitly without a socket", func(t *testing.T) {
		h := newHarness(&config.Config{HomeSet: true})
		require.NoError(t, h.run("bar"))

		t.Run("it should scan directly", func(t *testing.T) {
			assert.Equal(t, []string{"once waybar direct=true"}, h.bar.calls)
		})
	})
}

func TestNotifyCLI(t *testing.T) {
	t.Run("when run", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("notify"))

		t.Run("it should start the notifier", func(t *testing.T) {
			assert.Equal(t, 1, h.notify.runs)
		})
	})
}
