package harness_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/harness"
	"botropolis.city/botropolis/pkg/harness/codex"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ledgerCWD   = "/home/avesta/workspaces/github/mCedar/cinders"
	claudeSid   = "aaaaaaaa-0000-0000-0000-000000000097"
	codexSid    = "019a3c2e-5a1b-7c3d-9e4f-0a1b2c3d4e5f"
	codexOutput = `{"timestamp":"2026-09-17T07:00:00.000Z","type":"session_meta","payload":{"id":"` + codexSid + `","timestamp":"2026-09-17T07:00:00.000Z","cwd":"` + ledgerCWD + `"}}
{"timestamp":"2026-09-17T07:00:02.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Fix the flaky queue test"}]}}
{"timestamp":"2026-09-17T07:00:21.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Done."}]}}
`
)

var probes = state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }}

type ledgerCity struct {
	multi  *harness.Multi
	ledger string
}

func newLedgerCity(t *testing.T, ledger state.Ledger) ledgerCity {
	t.Helper()
	home := helpers.NewHome(t)
	home.Transcript(claudeSid, ledgerCWD,
		helpers.UserPrompt(claudeSid, ledgerCWD, "2026-09-17T07:30:00.000Z"),
		helpers.AssistantReply(claudeSid, "msg_97", "2026-09-17T07:30:10.000Z"))
	codexHome := t.TempDir()
	dir := filepath.Join(codexHome, "sessions", "2026", "09", "17")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	rollout := filepath.Join(dir, "rollout-2026-09-17T07-00-00-019a3c2e.jsonl")
	require.NoError(t, os.WriteFile(rollout, []byte(codexOutput), 0o600))
	require.NoError(t, os.Chtimes(rollout, now.Add(-time.Hour), now.Add(-time.Hour)))
	path := filepath.Join(t.TempDir(), "sessions.json")
	if ledger != nil {
		require.NoError(t, ledger.Save(path))
	}
	loader := state.NewLoader(home.Path, probes).WithLedger(path)
	return ledgerCity{multi: harness.NewMulti(harness.NewClaude(loader), codex.NewLoader(codexHome)), ledger: path}
}

func (c ledgerCity) load(t *testing.T) state.Snapshot {
	t.Helper()
	snapshot, err := c.multi.Load(now)
	require.NoError(t, err)
	return snapshot
}

func (c ledgerCity) saved(t *testing.T) state.Ledger {
	t.Helper()
	ledger, err := state.LoadLedger(c.ledger)
	require.NoError(t, err)
	return ledger
}

func sessionsWith(snapshot state.Snapshot, id string) []state.Session {
	var found []state.Session
	for _, s := range snapshot.Sessions {
		if s.ID == id {
			found = append(found, s)
		}
	}
	return found
}

func onlySession(t *testing.T, snapshot state.Snapshot, id string) state.Session {
	t.Helper()
	found := sessionsWith(snapshot, id)
	require.Len(t, found, 1)
	return found[0]
}

// Item 97. Packing a session is recorded in the ledger, which only the
// Claude loader read, so a Codex session could not be packed up, and the
// ledger's entry for it came back as a Claude session with no
// conversation.
func TestPackingOverHarnesses(t *testing.T) {
	harnesses := []struct {
		name, id, other, recordedAs string
		lastActivity                time.Time
	}{
		{name: "claude", id: claudeSid, other: codexSid, recordedAs: "", lastActivity: time.Date(2026, 9, 17, 7, 30, 10, 0, time.UTC)},
		{name: "codex", id: codexSid, other: claudeSid, recordedAs: "codex", lastActivity: time.Date(2026, 9, 17, 7, 0, 21, 0, time.UTC)},
	}
	for _, h := range harnesses {
		t.Run("when the ledger has the "+h.name+" session packed up", func(t *testing.T) {
			packedAt := h.lastActivity.Add(10 * time.Minute)
			snapshot := newLedgerCity(t, state.Ledger{h.id: {CWD: ledgerCWD, Harness: h.recordedAs, PackedAt: packedAt}}).load(t)

			t.Run("it should mark it packed", func(t *testing.T) {
				assert.True(t, onlySession(t, snapshot, h.id).Packed)
			})

			t.Run("it should carry when it was packed", func(t *testing.T) {
				assert.True(t, packedAt.Equal(onlySession(t, snapshot, h.id).PackedAt))
			})

			t.Run("it should leave the other harness's session unpacked", func(t *testing.T) {
				assert.False(t, onlySession(t, snapshot, h.other).Packed)
			})
		})

		t.Run("when the "+h.name+" session is packed up through the daemon", func(t *testing.T) {
			city := newLedgerCity(t, nil)
			city.load(t)
			require.NoError(t, city.multi.Pack(h.id, true, now))

			t.Run("and the city loads again", func(t *testing.T) {
				snapshot := city.load(t)

				t.Run("it should mark it packed", func(t *testing.T) {
					assert.True(t, onlySession(t, snapshot, h.id).Packed)
				})
			})

			t.Run("and then unpacked, and the city loads again", func(t *testing.T) {
				require.NoError(t, city.multi.Pack(h.id, false, now.Add(time.Minute)))
				snapshot := city.load(t)

				t.Run("it should not be packed", func(t *testing.T) {
					assert.False(t, onlySession(t, snapshot, h.id).Packed)
				})
			})
		})

		t.Run("when a packed-up "+h.name+" session does something new", func(t *testing.T) {
			city := newLedgerCity(t, state.Ledger{h.id: {CWD: ledgerCWD, Harness: h.recordedAs, PackedAt: h.lastActivity.Add(-10 * time.Minute)}})
			snapshot := city.load(t)

			t.Run("it should come back to the map", func(t *testing.T) {
				assert.False(t, onlySession(t, snapshot, h.id).Packed)
			})

			t.Run("it should record the unpack in the ledger", func(t *testing.T) {
				assert.False(t, city.saved(t)[h.id].Packed())
			})
		})

		t.Run("when the "+h.name+" session was demolished and has done nothing since", func(t *testing.T) {
			snapshot := newLedgerCity(t, state.Ledger{h.id: {CWD: ledgerCWD, Harness: h.recordedAs, DemolishedAt: h.lastActivity.Add(10 * time.Minute)}}).load(t)

			t.Run("it should stay off the map", func(t *testing.T) {
				assert.Empty(t, sessionsWith(snapshot, h.id))
			})
		})

		t.Run("when the city loads with the "+h.name+" session", func(t *testing.T) {
			city := newLedgerCity(t, nil)
			snapshot := city.load(t)

			t.Run("it should record it in the ledger under its harness", func(t *testing.T) {
				assert.Equal(t, h.recordedAs, city.saved(t)[h.id].Harness)
			})

			t.Run("it should show it exactly once", func(t *testing.T) {
				assert.Len(t, sessionsWith(snapshot, h.id), 1)
			})

			t.Run("and loads again", func(t *testing.T) {
				again := city.load(t)

				t.Run("it should still show it exactly once", func(t *testing.T) {
					assert.Len(t, sessionsWith(again, h.id), 1)
				})
			})
		})
	}
}
