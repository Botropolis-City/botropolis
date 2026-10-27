package acceptance_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/control"
)

// c on a district starts a session and opens a terminal on it, in the
// district's directory, running only claude attach (item 302). The
// claude here answers at once, so what is measured is the city's own
// hand-off; a real claude --bg takes seconds of its own.

const handoffJob = "1a2b3c4d"

type handoff struct {
	dir    string
	opened string
}

// newHandoff puts a claude that prints a job id at once, and a terminal
// that writes down what it was asked to run and where, ahead of
// everything else on PATH.
func newHandoff(tb testing.TB) handoff {
	tb.Helper()
	root := tb.TempDir()
	bin := filepath.Join(root, "bin")
	h := handoff{dir: filepath.Join(root, "district"), opened: filepath.Join(root, "opened")}
	require.NoError(tb, os.MkdirAll(bin, 0o755))
	require.NoError(tb, os.MkdirAll(h.dir, 0o755))
	script := func(name, body string) {
		require.NoError(tb, os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body), 0o755))
	}
	script("claude", "echo 'backgrounded · "+handoffJob+"'\n")
	script("fake-terminal", `printf '%s\n%s\n' "$PWD" "$*" > "`+h.opened+`.tmp" && mv "`+h.opened+`.tmp" "`+h.opened+`"`+"\n")
	tb.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	tb.Setenv("TERMINAL", "")
	return h
}

// press presses c on the district and waits for the terminal to open,
// returning what it was asked to run and where.
func (h handoff) press(tb testing.TB) (dir, command string) {
	tb.Helper()
	_ = os.Remove(h.opened)
	actor := commands.Actor{Sessions: commands.NewSessions(tb.TempDir(), control.Default("fake-terminal"))}
	require.NoError(tb, actor.Do(city.Action{Kind: city.ActionNew, Dir: h.dir}))
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if out, err := os.ReadFile(h.opened); err == nil {
			lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
			require.Len(tb, lines, 2)
			return lines[0], lines[1]
		}
		time.Sleep(time.Millisecond)
	}
	tb.Fatal("no terminal opened within 10s of c")
	return "", ""
}

func TestNewSessionOpens(t *testing.T) {
	t.Run("when c is pressed on a district", func(t *testing.T) {
		h := newHandoff(t)
		dir, command := h.press(t)

		t.Run("it should open a terminal running only claude attach on the new job", func(t *testing.T) {
			assert.Equal(t, "claude attach "+handoffJob, command)
		})

		t.Run("it should open it in the district's directory", func(t *testing.T) {
			assert.Equal(t, h.dir, dir)
		})
	})
}

// BenchmarkNewSessionHandOff reports the wall time from c to the
// terminal opening, with a claude that answers at once. It reports and
// never gates. Item 302.
func BenchmarkNewSessionHandOff(b *testing.B) {
	h := newHandoff(b)
	for b.Loop() {
		h.press(b)
	}
}
