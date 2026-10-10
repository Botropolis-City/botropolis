package demo

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestDirector(t *testing.T) {
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)
	corpus := newCorpus(t)
	transcript := func(home string) string {
		return filepath.Join(home, ".claude", "projects", claude.ProjectFolder(cwd), recordedID+".jsonl")
	}
	type run struct {
		home   string
		killed *[]int
		d      *Director
	}
	direct := func(t *testing.T, p Placement) run {
		t.Helper()
		home := t.TempDir()
		next := 5000
		spawn := func() (int, error) { next++; return next, nil }
		r := run{home: home, killed: &[]int{}}
		d, err := NewDirector(corpus, Scenario{Name: "s", Sessions: []Placement{p}}, home, now, spawn)
		require.NoError(t, err)
		d.kill = func(pid int) { *r.killed = append(*r.killed, pid) }
		r.d = d
		return r
	}
	until := func(t *testing.T, r run, offset time.Duration) {
		t.Helper()
		for _, e := range r.d.Events() {
			if e.At <= offset {
				require.NoError(t, r.d.Apply(e, now.Add(e.At)))
			}
		}
	}

	t.Run("when a session arrives partway through", func(t *testing.T) {
		r := direct(t, Placement{Ref: "tidepool/6f1d", State: state.Working, Arrive: Duration(6 * time.Second)})

		t.Run("it should not be there at the start", func(t *testing.T) {
			assert.NoFileExists(t, transcript(r.home))
		})

		t.Run("and its moment comes", func(t *testing.T) {
			until(t, r, 6*time.Second)

			t.Run("it should be staged then", func(t *testing.T) {
				assert.Len(t, records(t, r.home), 1)
			})
		})
	})

	t.Run("when a session leaves partway through", func(t *testing.T) {
		r := direct(t, Placement{Ref: "tidepool/6f1d", State: state.Working, Leave: Duration(20 * time.Second)})
		until(t, r, 20*time.Second)

		t.Run("it should end its stand-in, which parks it", func(t *testing.T) {
			assert.Equal(t, []int{5001}, *r.killed)
		})
	})

	t.Run("when a session changes state partway through", func(t *testing.T) {
		r := direct(t, Placement{Ref: "tidepool/6f1d", State: state.Working,
			Changes: []Change{{At: Duration(10 * time.Second), State: state.NeedsYou}}})
		until(t, r, 10*time.Second)

		t.Run("it should rewrite the record the loader derives the state from", func(t *testing.T) {
			assert.Equal(t, "idle", records(t, r.home)[0]["status"])
		})
	})

	t.Run("when an open PR is merged partway through", func(t *testing.T) {
		r := direct(t, Placement{Ref: "tidepool/6f1d", State: state.Working, PRs: []string{"open"},
			Changes: []Change{{At: Duration(12 * time.Second), PRs: []string{"merged"}}}})
		until(t, r, 12*time.Second)
		tr, err := claude.ReadTranscript(transcript(r.home))
		require.NoError(t, err)

		t.Run("it should be the same PR, merged, which is what the city celebrates", func(t *testing.T) {
			assert.Equal(t, []claude.PR{{Number: tr.PRs[0].Number, URL: tr.PRs[0].URL, Repository: "botropolis-demo/tidepool", State: claude.PRMerged}}, tr.PRs)
		})
	})

	t.Run("when events are listed", func(t *testing.T) {
		r := direct(t, Placement{Ref: "tidepool/6f1d", State: state.Working, Leave: Duration(20 * time.Second),
			Changes: []Change{{At: Duration(3 * time.Second), State: state.NeedsYou}}})
		var at []time.Duration
		for _, e := range r.d.Events() {
			at = append(at, e.At)
		}

		t.Run("it should list them in the order they happen", func(t *testing.T) {
			assert.Equal(t, []time.Duration{3 * time.Second, 20 * time.Second}, at)
		})
	})
}
