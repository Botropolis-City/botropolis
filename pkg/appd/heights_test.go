package appd

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 109: the daemon serves its first snapshot without waiting for the
// whole read, and the storeys it measures reach the map after it.
func TestDaemonStartup(t *testing.T) {
	const sid, cwd = "eeeeeeee-0000-0000-0000-000000000005", "/w/heavy"
	dead := state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}
	home := helpers.NewHome(t)
	start := time.Now().Add(-7 * 24 * time.Hour)
	lines := []string{helpers.UserPrompt(sid, cwd, start.Format(time.RFC3339Nano))}
	for i := range 1000 {
		lines = append(lines, helpers.AssistantReply(sid, fmt.Sprintf("msg_%04d", i), start.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano)))
	}
	home.Transcript(sid, cwd, lines...)
	find := func(snapshot state.Snapshot) state.Session {
		for _, s := range snapshot.Sessions {
			if s.ID == sid {
				return s
			}
		}
		return state.Session{}
	}

	t.Run("when the city loads directly, with no daemon", func(t *testing.T) {
		small := helpers.NewHome(t)
		for i := range 3 {
			id := fmt.Sprintf("ffffffff-0000-0000-0000-00000000000%d", i)
			small.Transcript(id, cwd, helpers.UserPrompt(id, cwd, start.Format(time.RFC3339Nano)),
				helpers.AssistantReply(id, "msg_"+id[len(id)-4:], start.Add(time.Second).Format(time.RFC3339Nano)))
		}
		snapshot, err := Harnesses(&config.Config{Home: small.Path, HomeSet: true}, dead, Paced).Load(time.Now())
		require.NoError(t, err)

		t.Run("it should draw a home whose sessions are all read on its own scale", func(t *testing.T) {
			assert.True(t, snapshot.HeftsRead)
		})
	})

	t.Run("when the home has a history of stopped sessions", func(t *testing.T) {
		d := newDaemon(&config.Config{Home: home.Path, HomeSet: true}, dead)
		require.NoError(t, d.Rescan())
		first := d.Snapshot()

		t.Run("it should serve the first snapshot before the whole read finishes", func(t *testing.T) {
			assert.True(t, find(first).HeftPartial)
		})

		t.Run("and the whole read finishes", func(t *testing.T) {
			deadline := time.Now().Add(30 * time.Second)
			for !d.Snapshot().HeftsRead && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}

			t.Run("it should rescan with the session's exact heft", func(t *testing.T) {
				assert.Equal(t, int64(1000*(2+5+25_000)), find(d.Snapshot()).Heft)
			})
		})
	})
}
