package state_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 205: a rescan stat'ed every stopped transcript, which among 2,000
// sessions was a sixth of it. A directory the watcher holds is trusted to
// say which of its transcripts changed; a sixtieth of the rest is still
// looked at each load, so a change the watcher missed is read within 60
// loads, and every transcript is looked at with no watcher, after the
// watcher overflowed, or in a directory it could not watch.
func TestCatalogueStats(t *testing.T) {
	const perDir = 60
	dirs := []string{"/w/stats-a", "/w/stats-b"}
	stamp := func(at time.Time) string { return at.Format("2006-01-02T15:04:05.000Z") }
	id := func(d, i int) string { return fmt.Sprintf("00000000-0000-4205-8%03d-%012d", d, i) }
	at := now
	type fixture struct {
		home     *helpers.Home
		loader   *state.Loader
		paths    [][]string
		projects []string
	}
	load := func(t *testing.T, f fixture) state.Snapshot {
		t.Helper()
		at = at.Add(time.Second)
		snapshot, err := f.loader.Load(at)
		require.NoError(t, err)
		return snapshot
	}
	statsOf := func(t *testing.T, f fixture) int {
		t.Helper()
		before := f.loader.TranscriptStats()
		load(t, f)
		return f.loader.TranscriptStats() - before
	}
	setup := func(t *testing.T, watched ...bool) fixture {
		t.Helper()
		f := fixture{home: helpers.NewHome(t)}
		for d, cwd := range dirs {
			var paths []string
			for i := 0; i < perDir; i++ {
				ts := stamp(now.Add(-time.Duration(d*perDir+i) * time.Hour))
				paths = append(paths, f.home.Transcript(id(d, i), cwd,
					helpers.UserPrompt(id(d, i), cwd, ts), helpers.AssistantReply(id(d, i), fmt.Sprintf("msg_%d_%d", d, i), ts)))
			}
			f.paths = append(f.paths, paths)
			f.projects = append(f.projects, filepath.Join(f.home.ClaudeDir(), "projects", helpers.ProjectSlug(cwd)))
		}
		f.loader = state.NewLoader(f.home.Path, alive)
		load(t, f)
		for d, ok := range watched {
			f.loader.Watching(f.projects[d], ok)
		}
		load(t, f)
		return f
	}
	write := func(t *testing.T, f fixture, d, i int) time.Time {
		t.Helper()
		when := now.Add(time.Duration(d*perDir+i) * time.Minute)
		f.home.AppendTranscript(id(d, i), dirs[d], helpers.AssistantReply(id(d, i), fmt.Sprintf("msg_%d_%d_more", d, i), stamp(when)))
		return when.Truncate(time.Millisecond)
	}
	lastActivity := func(t *testing.T, snapshot state.Snapshot, d, i int) time.Time {
		t.Helper()
		return sessionByID(t, snapshot, id(d, i)).LastActivity
	}
	audit := 2 * perDir / 60

	t.Run("when the watcher holds every directory", func(t *testing.T) {
		t.Run("and it says one stopped transcript changed", func(t *testing.T) {
			f := setup(t, true, true)
			when := write(t, f, 0, 7)
			f.loader.Changed(f.paths[0][7])
			before := f.loader.TranscriptStats()
			snapshot := load(t, f)

			t.Run("it should stat only that transcript and the audit's share", func(t *testing.T) {
				assert.LessOrEqual(t, f.loader.TranscriptStats()-before, 1+audit)
			})

			t.Run("it should read the change", func(t *testing.T) {
				assert.Equal(t, when, lastActivity(t, snapshot, 0, 7))
			})
		})

		t.Run("and it says nothing changed", func(t *testing.T) {
			f := setup(t, true, true)

			t.Run("it should stat only the audit's share", func(t *testing.T) {
				assert.Equal(t, audit, statsOf(t, f))
			})
		})

		t.Run("and it overflowed", func(t *testing.T) {
			f := setup(t, true, true)
			f.loader.Unwatched()

			t.Run("it should stat every stopped transcript on the next load", func(t *testing.T) {
				assert.Equal(t, 2*perDir, statsOf(t, f))
			})
		})

		t.Run("and it missed a change", func(t *testing.T) {
			f := setup(t, true, true)
			when := write(t, f, 1, 33)
			var snapshot state.Snapshot
			for i := 0; i < 60; i++ {
				snapshot = load(t, f)
			}

			t.Run("it should read it within 60 loads", func(t *testing.T) {
				assert.Equal(t, when, lastActivity(t, snapshot, 1, 33))
			})
		})
	})

	t.Run("when the watcher holds every directory, and a stopped session it trusts has a subagent that worked elsewhere", func(t *testing.T) {
		f := setup(t, true, true)
		ts := stamp(now.Add(-3 * time.Hour))
		f.home.Subagent(id(0, 10), dirs[0], "a205", "", helpers.UserPrompt(id(0, 10), "/w/elsewhere", ts))
		f.loader.Changed(f.paths[0][10])
		load(t, f)
		snapshot := load(t, f)

		t.Run("it should keep the subagent's visit", func(t *testing.T) {
			assert.Contains(t, sessionByID(t, snapshot, id(0, 10)).Visits, "/w/elsewhere")
		})
	})

	t.Run("when the watcher loses a directory it held", func(t *testing.T) {
		f := setup(t, true, true)
		f.loader.Watching(f.projects[1], false)

		t.Run("it should stat that directory's transcripts every load", func(t *testing.T) {
			assert.Equal(t, perDir+perDir/60, statsOf(t, f))
		})
	})

	t.Run("when the loader has no watcher", func(t *testing.T) {
		f := setup(t)

		t.Run("it should stat every stopped transcript every load", func(t *testing.T) {
			assert.Equal(t, 2*perDir, statsOf(t, f))
		})
	})

	t.Run("when a watch could not be added for one directory", func(t *testing.T) {
		f := setup(t, true, false)

		t.Run("it should stat that directory's transcripts every load", func(t *testing.T) {
			assert.Equal(t, perDir+perDir/60, statsOf(t, f))
		})

		t.Run("and it changes with nothing said", func(t *testing.T) {
			when := write(t, f, 1, 5)

			t.Run("it should read the change on the next load", func(t *testing.T) {
				assert.Equal(t, when, lastActivity(t, load(t, f), 1, 5))
			})
		})
	})

	t.Run("when a directory has just been watched", func(t *testing.T) {
		f := setup(t, true)
		f.loader.Watching(f.projects[1], true)

		t.Run("it should stat its transcripts once more, for what changed before the watch", func(t *testing.T) {
			assert.Equal(t, perDir+perDir/60, statsOf(t, f))
		})
	})
}
