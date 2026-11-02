package commands_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/state"
)

type snapshotOf state.Snapshot

func (s snapshotOf) Snapshot(bool) (state.Snapshot, error) { return state.Snapshot(s), nil }

// Item 333: status counts what the quiet sweep put in the depot.
func TestStatusCountsTheSweep(t *testing.T) {
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	run := func(t *testing.T, all, asJSON bool, sessions ...state.Session) string {
		t.Helper()
		var out bytes.Buffer
		require.NoError(t, commands.NewStatus(snapshotOf{At: now, Sessions: sessions}).Run(&out, false, all, asJSON))
		return out.String()
	}
	working := state.Session{ID: "w", CWD: "/w/lib", State: state.Working, LastActivity: now}
	swept := func(id string) state.Session {
		return state.Session{ID: id, CWD: "/w/" + id, State: state.Resting, Packed: true, Swept: true, LastActivity: now.Add(-10 * 24 * time.Hour)}
	}
	byHand := state.Session{ID: "h", CWD: "/w/h", State: state.Resting, Packed: true, LastActivity: now}

	for _, all := range []bool{false, true} {
		t.Run("when the sweep has sessions in the depot, and --all is "+map[bool]string{false: "off", true: "on"}[all], func(t *testing.T) {
			out := run(t, all, false, working, swept("a"), swept("b"), byHand)

			t.Run("it should say how many, and how to bring them back", func(t *testing.T) {
				assert.Contains(t, out, "2 sessions packed up by the quiet sweep are in the depot; unpack any from the map\n")
			})
		})
	}

	t.Run("when one swept session is in the depot", func(t *testing.T) {
		out := run(t, false, false, working, swept("a"))

		t.Run("it should say one session", func(t *testing.T) {
			assert.Contains(t, out, "1 session packed up by the quiet sweep is in the depot")
		})
	})

	t.Run("when nothing was swept", func(t *testing.T) {
		out := run(t, false, false, working, byHand)

		t.Run("it should say nothing of the sweep", func(t *testing.T) {
			assert.NotContains(t, out, "quiet sweep")
		})
	})

	t.Run("when the output is JSON", func(t *testing.T) {
		out := run(t, true, true, working, swept("a"))

		t.Run("it should stay one JSON array, the count in each session's swept key", func(t *testing.T) {
			assert.NotContains(t, out, "quiet sweep")
		})
	})
}
