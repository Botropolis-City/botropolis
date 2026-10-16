package state_test

import (
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	loaderCWD = "/home/avesta/workspaces/github/mCedar/cinders"
	otherSid  = "ffffffff-0000-0000-0000-000000000000"
)

func liveHome(t *testing.T) *helpers.Home {
	t.Helper()
	home := helpers.NewHome(t).Session(4242, sidA, loaderCWD, "interactive", "idle")
	home.Transcript(sidA, loaderCWD,
		helpers.UserPrompt(sidA, loaderCWD, "2026-09-16T19:50:00.000Z"),
		helpers.AssistantReply(sidA, "msg_01", "2026-09-16T19:50:10.000Z"))
	home.Transcript(otherSid, loaderCWD,
		helpers.UserPrompt(otherSid, loaderCWD, "2026-09-16T18:00:00.000Z"),
		helpers.AssistantReply(otherSid, "msg_09", "2026-09-16T18:00:10.000Z"))
	return home
}

func TestLoader(t *testing.T) {
	t.Run("when a home holds one live session and one old transcript", func(t *testing.T) {
		home := liveHome(t)
		loader := state.NewLoader(home.Path, alive)
		snapshot, err := loader.Load(now)
		require.NoError(t, err)

		t.Run("it should build the live session and catalogue the other as parked", func(t *testing.T) {
			require.Len(t, snapshot.Sessions, 2)
			assert.Equal(t, state.NeedsYou, sessionByID(t, snapshot, sidA).State)
			assert.Equal(t, state.Resting, sessionByID(t, snapshot, otherSid).State)
		})

		t.Run("it should read each transcript once", func(t *testing.T) {
			assert.Equal(t, 2, loader.Reads())
		})

		t.Run("and it loads again with nothing changed", func(t *testing.T) {
			_, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)

			t.Run("it should not read the transcripts again", func(t *testing.T) {
				assert.Equal(t, 2, loader.Reads())
			})
		})

		t.Run("and the transcript grows", func(t *testing.T) {
			home.AppendTranscript(sidA, loaderCWD,
				helpers.UserPrompt(sidA, loaderCWD, "2026-09-16T19:51:00.000Z"))
			snapshot, err := loader.Load(now.Add(2 * time.Second))
			require.NoError(t, err)

			t.Run("it should read it again", func(t *testing.T) {
				assert.Equal(t, 3, loader.Reads())
			})

			t.Run("it should reflect the new tail", func(t *testing.T) {
				assert.Equal(t, claude.TurnWorking, sessionByID(t, snapshot, sidA).Turn)
			})
		})
	})

	t.Run("when the home has a stats cache", func(t *testing.T) {
		home := liveHome(t).StatsCache(`{"version":4,"lastComputedDate":"2026-07-02","dailyActivity":[],"dailyModelTokens":[],` +
			`"modelUsage":{},"totalSessions":513,"totalMessages":125894,"firstSessionDate":"2026-01-13T17:49:58.240Z","hourCounts":{}}`)
		snapshot, err := state.NewLoader(home.Path, alive).Load(now)
		require.NoError(t, err)

		t.Run("it should carry the rollup in the snapshot", func(t *testing.T) {
			require.NotNil(t, snapshot.Stats)
			assert.Equal(t, 513, snapshot.Stats.TotalSessions)
		})
	})

	t.Run("when the home has no stats cache", func(t *testing.T) {
		snapshot, err := state.NewLoader(liveHome(t).Path, alive).Load(now)
		require.NoError(t, err)

		t.Run("it should carry no rollup", func(t *testing.T) {
			assert.Nil(t, snapshot.Stats)
		})
	})

	t.Run("when a live record has no transcript on disk", func(t *testing.T) {
		home := helpers.NewHome(t).Session(4242, sidA, loaderCWD, "interactive", "busy")
		snapshot, err := state.NewLoader(home.Path, alive).Load(now)
		require.NoError(t, err)

		t.Run("it should still build the session from the record", func(t *testing.T) {
			require.Len(t, snapshot.Sessions, 1)
			assert.Equal(t, state.Working, snapshot.Sessions[0].State)
		})
	})

	t.Run("when the live session has subagents", func(t *testing.T) {
		home := liveHome(t)
		home.Subagent(sidA, loaderCWD, "a2f854e70", `{"agentType":"Explore","toolUseId":"toolu_01"}`,
			helpers.UserPrompt(sidA, loaderCWD, "2026-09-16T19:50:05.000Z"),
			helpers.AssistantReply(sidA, "msg_sub", "2026-09-16T19:50:06.000Z"))
		loader := state.NewLoader(home.Path, alive)
		snapshot, err := loader.Load(now)
		require.NoError(t, err)

		t.Run("it should count them", func(t *testing.T) {
			assert.Equal(t, 1, sessionByID(t, snapshot, sidA).Subagents)
		})

		t.Run("it should read transcript, subagent, and the parked transcript once each", func(t *testing.T) {
			assert.Equal(t, 3, loader.Reads())
		})

		t.Run("and it loads again with nothing changed", func(t *testing.T) {
			_, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)

			t.Run("it should read nothing", func(t *testing.T) {
				assert.Equal(t, 3, loader.Reads())
			})
		})
	})
}

func TestCatalogue(t *testing.T) {
	old := "2026-09-10T12:00:00.000Z"
	parkedHome := func(t *testing.T) *helpers.Home {
		t.Helper()
		home := liveHome(t)
		home.Transcript("aaaaaaaa-0000-0000-0000-000000000001", loaderCWD,
			helpers.UserPrompt("aaaaaaaa-0000-0000-0000-000000000001", loaderCWD, old),
			helpers.AssistantReply("aaaaaaaa-0000-0000-0000-000000000001", "msg_p1", "2026-09-10T12:00:10.000Z"),
			helpers.AITitle("aaaaaaaa-0000-0000-0000-000000000001", "An old job"))
		return home
	}

	t.Run("when a home holds a transcript with no live record", func(t *testing.T) {
		home := parkedHome(t)
		loader := state.NewLoader(home.Path, alive)
		snapshot, err := loader.Load(now)
		require.NoError(t, err)

		t.Run("it should list it as a resting session", func(t *testing.T) {
			require.Len(t, snapshot.Sessions, 3)
			parked := sessionByID(t, snapshot, "aaaaaaaa-0000-0000-0000-000000000001")
			assert.Equal(t, state.Resting, parked.State)
		})

		t.Run("it should carry the transcript's title", func(t *testing.T) {
			assert.Equal(t, "An old job", sessionByID(t, snapshot, "aaaaaaaa-0000-0000-0000-000000000001").Title)
		})

		t.Run("it should date it by its last activity", func(t *testing.T) {
			assert.Equal(t, "2026-09-10T12:00:10Z", sessionByID(t, snapshot, "aaaaaaaa-0000-0000-0000-000000000001").LastActivity.Format("2006-01-02T15:04:05Z"))
		})

		t.Run("it should not be alive", func(t *testing.T) {
			assert.False(t, sessionByID(t, snapshot, "aaaaaaaa-0000-0000-0000-000000000001").Alive)
		})

		t.Run("and it loads again with nothing changed", func(t *testing.T) {
			before := loader.Reads()
			_, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)

			t.Run("it should read nothing again", func(t *testing.T) {
				assert.Equal(t, before, loader.Reads())
			})
		})
	})

	t.Run("when a stopped session's transcript is older than seven days", func(t *testing.T) {
		home := parkedHome(t)
		snapshot, err := state.NewLoader(home.Path, alive).Load(time.Date(2026, time.October, 30, 8, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		t.Run("it should still be on the snapshot", func(t *testing.T) {
			assert.Len(t, snapshot.Sessions, 3)
		})
	})

	t.Run("when the loader keeps a ledger", func(t *testing.T) {
		home := parkedHome(t)
		path := filepath.Join(t.TempDir(), "sessions.json")
		_, err := state.NewLoader(home.Path, alive).WithLedger(path).Load(now)
		require.NoError(t, err)
		ledger, err := state.LoadLedger(path)
		require.NoError(t, err)

		t.Run("it should record every session it shows", func(t *testing.T) {
			assert.Len(t, ledger, 3)
		})
	})

	t.Run("when a session the city has shown has lost its transcript", func(t *testing.T) {
		gone := "dddddddd-0000-0000-0000-000000000004"
		first, last := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC), time.Date(2026, 8, 2, 17, 30, 0, 0, time.UTC)
		ledgerWith := func(t *testing.T) string {
			t.Helper()
			path := filepath.Join(t.TempDir(), "sessions.json")
			require.NoError(t, state.Ledger{gone: {CWD: "/where/it/was", StartedAt: first, LastActivity: first.Add(time.Hour)}}.Save(path))
			return path
		}

		t.Run("and the prompt history has it", func(t *testing.T) {
			home := liveHome(t)
			home.Prompt(gone, loaderCWD, "make the queue retry", first).Prompt(gone, loaderCWD, "ship it", last)
			snapshot, err := state.NewLoader(home.Path, alive).WithLedger(ledgerWith(t)).Load(now)
			require.NoError(t, err)
			s := sessionByID(t, snapshot, gone)

			t.Run("it should be resting", func(t *testing.T) {
				assert.Equal(t, state.Resting, s.State)
			})

			t.Run("it should take its working directory from the history", func(t *testing.T) {
				assert.Equal(t, loaderCWD, s.CWD)
			})

			t.Run("it should take its last activity from its last prompt", func(t *testing.T) {
				assert.True(t, last.Equal(s.LastActivity))
			})

			t.Run("it should take its title from its first prompt", func(t *testing.T) {
				assert.Equal(t, "make the queue retry", s.Title)
			})

			t.Run("it should keep its last prompt", func(t *testing.T) {
				assert.Equal(t, "ship it", s.LastPrompt)
			})

			t.Run("it should be marked as having no conversation", func(t *testing.T) {
				assert.True(t, s.ConversationGone)
			})
		})

		t.Run("and the history does not have it either", func(t *testing.T) {
			home := liveHome(t)
			snapshot, err := state.NewLoader(home.Path, alive).WithLedger(ledgerWith(t)).Load(now)
			require.NoError(t, err)
			s := sessionByID(t, snapshot, gone)

			t.Run("it should take its working directory from the ledger", func(t *testing.T) {
				assert.Equal(t, "/where/it/was", s.CWD)
			})

			t.Run("it should take its last activity from the ledger", func(t *testing.T) {
				assert.True(t, first.Add(time.Hour).Equal(s.LastActivity))
			})

			t.Run("it should be marked as having no conversation", func(t *testing.T) {
				assert.True(t, s.ConversationGone)
			})
		})
	})

	t.Run("when the ledger has a session packed up", func(t *testing.T) {
		home := parkedHome(t)
		path := filepath.Join(t.TempDir(), "sessions.json")
		packed := "aaaaaaaa-0000-0000-0000-000000000001"
		require.NoError(t, state.Ledger{packed: {CWD: loaderCWD, PackedAt: now.Add(-time.Hour)}}.Save(path))
		snapshot, err := state.NewLoader(home.Path, alive).WithLedger(path).Load(now)
		require.NoError(t, err)

		t.Run("it should mark it packed", func(t *testing.T) {
			assert.True(t, sessionByID(t, snapshot, packed).Packed)
		})

		t.Run("it should leave the others unpacked", func(t *testing.T) {
			assert.False(t, sessionByID(t, snapshot, sidA).Packed)
		})
	})

	t.Run("when a session's pull request has merged", func(t *testing.T) {
		merged := "eeeeeeee-0000-0000-0000-000000000005"
		withMerge := func(t *testing.T, live bool) *helpers.Home {
			t.Helper()
			home := liveHome(t)
			if live {
				home.Session(5151, merged, loaderCWD, "interactive", "busy")
			}
			home.Transcript(merged, loaderCWD,
				helpers.UserPrompt(merged, loaderCWD, "2026-09-16T10:00:00.000Z"),
				`{"type":"pr-link","sessionId":"`+merged+`","prNumber":7,"prUrl":"https://example.com/pull/7","prRepository":"o/r","timestamp":"2026-09-16T10:01:00.000Z"}`,
				`{"type":"user","sessionId":"`+merged+`","timestamp":"2026-09-16T10:02:00.000Z","pr":{"number":7,"url":"https://example.com/pull/7","action":"merged"},"message":{"role":"user","content":"merged"}}`)
			return home
		}

		t.Run("and the session has stopped", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sessions.json")
			snapshot, err := state.NewLoader(withMerge(t, false).Path, alive).WithLedger(path).Load(now)
			require.NoError(t, err)
			ledger, err := state.LoadLedger(path)
			require.NoError(t, err)

			t.Run("it should pack it up", func(t *testing.T) {
				assert.True(t, sessionByID(t, snapshot, merged).Packed)
			})

			t.Run("it should record the pack-up in the ledger", func(t *testing.T) {
				assert.True(t, ledger[merged].Packed())
			})
		})

		t.Run("and the session is still live", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sessions.json")
			snapshot, err := state.NewLoader(withMerge(t, true).Path, alive).WithLedger(path).Load(now)
			require.NoError(t, err)

			t.Run("it should not pack it up yet", func(t *testing.T) {
				assert.False(t, sessionByID(t, snapshot, merged).Packed)
			})
		})

		t.Run("and it was unpacked after the merge", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sessions.json")
			require.NoError(t, state.Ledger{merged: {CWD: loaderCWD, PackedAt: now.Add(-2 * time.Hour), UnpackedAt: now.Add(-time.Hour)}}.Save(path))
			snapshot, err := state.NewLoader(withMerge(t, false).Path, alive).WithLedger(path).Load(now)
			require.NoError(t, err)

			t.Run("it should not pack it up again", func(t *testing.T) {
				assert.False(t, sessionByID(t, snapshot, merged).Packed)
			})
		})
	})

	t.Run("when a transcript with no live record is a bridge stub", func(t *testing.T) {
		home := liveHome(t)
		home.Transcript("bbbbbbbb-0000-0000-0000-000000000002", loaderCWD,
			`{"type":"bridge-session","sessionId":"bbbbbbbb-0000-0000-0000-000000000002","bridgeSessionId":"x"}`)
		snapshot, err := state.NewLoader(home.Path, alive).Load(now)
		require.NoError(t, err)

		t.Run("it should leave it out", func(t *testing.T) {
			for _, s := range snapshot.Sessions {
				assert.NotEqual(t, "bbbbbbbb-0000-0000-0000-000000000002", s.ID)
			}
		})
	})
}

func sessionByID(t *testing.T, snapshot state.Snapshot, id string) state.Session {
	t.Helper()
	for _, s := range snapshot.Sessions {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("no session %s in snapshot", id)
	return state.Session{}
}

// Item 49. `claude -p "/usage"` writes a transcript on every run, and
// the city is built out of transcripts, so a usage poller running in an
// ordinary directory adds a resting session to the city each time it
// asks the city what its usage is. Measured on the real machine before
// this guard: three polls took it from 22 parked to 25.
func TestUsageProbeIsNotASession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	probe := claude.UsageProbeDir()
	home := liveHome(t)
	const probeID = "cccccccc-0000-0000-0000-000000000009"
	home.Transcript(probeID, probe,
		helpers.UserPrompt(probeID, probe, "2026-09-10T12:00:00.000Z"),
		helpers.AssistantReply(probeID, "msg_probe", "2026-09-10T12:00:02.000Z"))

	loader := state.NewLoader(home.Path, alive)
	snapshot, err := loader.Load(now)
	require.NoError(t, err)

	t.Run("when the usage probe has left a transcript of its own", func(t *testing.T) {
		t.Run("it should not become a session in the city it measures", func(t *testing.T) {
			for _, s := range snapshot.Sessions {
				assert.NotEqual(t, probeID, s.ID, "the probe was counted as a session")
			}
		})

		t.Run("it should leave the real sessions alone", func(t *testing.T) {
			assert.NotEmpty(t, snapshot.Sessions)
		})
	})
}
