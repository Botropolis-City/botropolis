package stats_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

func TestNotePruned(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(stateDir) stateDir
	}{
		{"with data kept", func(d stateDir) stateDir { return d.withDatabase().withMarker() }},
		{"and the off prompt still owed", func(d stateDir) stateDir { return d.withDatabase() }},
	} {
		t.Run("when stats are off "+c.name+", and prune removes a session", func(t *testing.T) {
			d := c.setup(newStateDir(t))
			require.NoError(t, stats.NotePruned(d.dir, sessionA))
			data, _ := os.ReadFile(d.notes())

			t.Run("it should note it", func(t *testing.T) {
				assert.Equal(t, sessionA+"\n", string(data))
			})
		})
	}

	t.Run("when the off prompt is still owed, and prune has noted a session", func(t *testing.T) {
		d := newStateDir(t).withDatabase()
		require.NoError(t, stats.NotePruned(d.dir, sessionA))

		t.Run("it should still owe the off prompt", func(t *testing.T) {
			assert.Equal(t, stats.OffPrompt, stats.Owed(d.dir, false))
		})
	})

	t.Run("when the data is deleted after prune noted a session", func(t *testing.T) {
		d := newStateDir(t).withDatabase()
		require.NoError(t, stats.NotePruned(d.dir, sessionA))
		require.NoError(t, stats.TurnOff(d.dir, false))

		t.Run("it should delete the notes", func(t *testing.T) {
			assert.False(t, exists(d.notes()))
		})
	})

	t.Run("when stats are off with nothing kept", func(t *testing.T) {
		d := newStateDir(t)
		require.NoError(t, stats.NotePruned(d.dir, sessionA))

		t.Run("it should note nothing", func(t *testing.T) {
			assert.False(t, exists(d.notes()))
		})
	})

	t.Run("when stats are turned back on keeping the old data", func(t *testing.T) {
		d := newStateDir(t).withDatabase().withMarker()
		s, err := stats.Open(d.db())
		require.NoError(t, err)
		_, err = s.DB().Exec("INSERT INTO sessions (id) VALUES (?), (?)", sessionA, sessionB)
		require.NoError(t, err)
		_, err = s.DB().Exec("INSERT INTO message_sessions (message_id, session_id) VALUES ('msg_1', ?)", sessionA)
		require.NoError(t, err)
		require.NoError(t, s.Close())
		lastWritten := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(d.db(), lastWritten, lastWritten))
		require.NoError(t, stats.NotePruned(d.dir, sessionA))
		require.NoError(t, stats.TurnOn(d.dir, true, time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)))

		t.Run("it should forget each session noted while off", func(t *testing.T) {
			assert.Equal(t, 0, d.count("SELECT count(*) FROM messages"))
		})

		t.Run("it should keep the sessions not noted", func(t *testing.T) {
			assert.Equal(t, 1, d.count("SELECT count(*) FROM sessions WHERE id = '"+sessionB+"'"))
		})

		t.Run("it should forget the noted sessions before recording the gap, from when the database was last written", func(t *testing.T) {
			assert.Equal(t, 1, d.count(fmt.Sprintf("SELECT count(*) FROM gaps WHERE off_at = %d", lastWritten.UnixMilli())))
		})
	})
}

func TestOffSince(t *testing.T) {
	t.Run("when the database was last written at a time", func(t *testing.T) {
		d := newStateDir(t).withDatabase()
		at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(d.db(), at, at))
		since, _ := stats.OffSince(d.dir)

		t.Run("it should be that time", func(t *testing.T) {
			assert.True(t, at.Equal(since))
		})
	})
}
