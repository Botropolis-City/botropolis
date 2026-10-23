package state_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 205: a session working moves its last activity on every rescan,
// and saving the whole ledger for it was a third of a rescan among 2,000
// sessions. Its only use is placing a session whose transcript is gone,
// so those changes are saved at most once a minute, and on shutdown;
// anything else is saved at once.
func TestLoaderSavesTheLedger(t *testing.T) {
	const working = "aaaaaaaa-0000-4205-8000-000000000001"
	start := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	stamp := func(at time.Time) string { return at.Format("2006-01-02T15:04:05.000Z") }
	reply := 0
	type fixture struct {
		home   *helpers.Home
		path   string
		loader *state.Loader
	}
	setup := func(t *testing.T) fixture {
		t.Helper()
		home := helpers.NewHome(t)
		home.Transcript(working, loaderCWD,
			helpers.UserPrompt(working, loaderCWD, stamp(start)),
			helpers.AssistantReply(working, "msg_205_0", stamp(start)))
		path := filepath.Join(t.TempDir(), "botropolis", "sessions.json")
		loader := state.NewLoader(home.Path, alive).WithLedger(path)
		_, err := loader.Load(start)
		require.NoError(t, err)
		return fixture{home: home, path: path, loader: loader}
	}
	work := func(t *testing.T, f fixture, at time.Time) {
		t.Helper()
		reply++
		f.home.AppendTranscript(working, loaderCWD, helpers.AssistantReply(working, fmt.Sprintf("msg_205_%d", reply), stamp(at)))
		_, err := f.loader.Load(at)
		require.NoError(t, err)
	}
	onDisk := func(t *testing.T, f fixture) state.Ledger {
		t.Helper()
		ledger, err := state.LoadLedger(f.path)
		require.NoError(t, err)
		return ledger
	}

	t.Run("when only a working session's last activity moved", func(t *testing.T) {
		t.Run("and less than a minute has passed since the last save", func(t *testing.T) {
			f := setup(t)
			work(t, f, start.Add(30*time.Second))

			t.Run("it should not write the ledger", func(t *testing.T) {
				assert.Equal(t, start, onDisk(t, f)[working].LastActivity)
			})
		})

		t.Run("and a minute has passed since the last save", func(t *testing.T) {
			f := setup(t)
			work(t, f, start.Add(30*time.Second))
			work(t, f, start.Add(61*time.Second))

			t.Run("it should write the ledger", func(t *testing.T) {
				assert.Equal(t, start.Add(61*time.Second), onDisk(t, f)[working].LastActivity)
			})
		})

		t.Run("and a minute passes with nothing more changing", func(t *testing.T) {
			f := setup(t)
			work(t, f, start.Add(30*time.Second))
			_, err := f.loader.Load(start.Add(61 * time.Second))
			require.NoError(t, err)

			t.Run("it should write the ledger on the next load", func(t *testing.T) {
				assert.Equal(t, start.Add(30*time.Second), onDisk(t, f)[working].LastActivity)
			})
		})

		t.Run("and the loader is flushed, as the daemon does on shutdown", func(t *testing.T) {
			f := setup(t)
			work(t, f, start.Add(30*time.Second))
			require.NoError(t, f.loader.Flush())

			t.Run("it should write the ledger", func(t *testing.T) {
				assert.Equal(t, start.Add(30*time.Second), onDisk(t, f)[working].LastActivity)
			})
		})

		t.Run("and the daemon is killed without shutting down", func(t *testing.T) {
			f := setup(t)
			at := start
			for i := 0; i < 37; i++ {
				at = at.Add(5 * time.Second)
				work(t, f, at)
			}

			t.Run("it should lose at most a minute of last-activity times", func(t *testing.T) {
				assert.LessOrEqual(t, at.Sub(onDisk(t, f)[working].LastActivity), time.Minute)
			})
		})
	})

	t.Run("when a new session appears within the minute", func(t *testing.T) {
		f := setup(t)
		f.home.Transcript(otherSid, loaderCWD,
			helpers.UserPrompt(otherSid, loaderCWD, stamp(start.Add(10*time.Second))),
			helpers.AssistantReply(otherSid, "msg_205_other", stamp(start.Add(10*time.Second))))
		_, err := f.loader.Load(start.Add(10 * time.Second))
		require.NoError(t, err)

		t.Run("it should write the ledger at once", func(t *testing.T) {
			assert.Contains(t, onDisk(t, f), otherSid)
		})
	})

	t.Run("when the file on disk is the one the loader last wrote", func(t *testing.T) {
		f := setup(t)
		before := f.loader.LedgerReads()
		work(t, f, start.Add(61*time.Second))

		t.Run("it should not read it again before writing", func(t *testing.T) {
			assert.Equal(t, before, f.loader.LedgerReads())
		})
	})

	t.Run("when a newer build that this one can read wrote the file since", func(t *testing.T) {
		f := setup(t)
		data, err := os.ReadFile(f.path)
		require.NoError(t, err)
		var file map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &file))
		file["version"], file["readableFrom"] = json.RawMessage("3"), json.RawMessage("2")
		data, err = json.Marshal(file)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(f.path, data, 0o600))
		work(t, f, start.Add(61*time.Second))
		require.NoError(t, json.Unmarshal(mustRead(t, f.path), &file))

		t.Run("it should keep that version", func(t *testing.T) {
			assert.Equal(t, "3", string(file["version"]))
		})
	})
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
