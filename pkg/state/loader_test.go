package state_test

import (
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
			assert.Equal(t, state.Parked, sessionByID(t, snapshot, otherSid).State)
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

		t.Run("it should list it as a parked session", func(t *testing.T) {
			require.Len(t, snapshot.Sessions, 3)
			parked := sessionByID(t, snapshot, "aaaaaaaa-0000-0000-0000-000000000001")
			assert.Equal(t, state.Parked, parked.State)
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

	t.Run("when a parked transcript is older than the catalogue keeps", func(t *testing.T) {
		home := parkedHome(t)
		loader := state.NewLoader(home.Path, alive).WithParkedMaxAge(48 * time.Hour)
		snapshot, err := loader.Load(time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		t.Run("it should leave it out", func(t *testing.T) {
			assert.Len(t, snapshot.Sessions, 2)
		})
	})

	t.Run("when the catalogue is switched off", func(t *testing.T) {
		home := parkedHome(t)
		snapshot, err := state.NewLoader(home.Path, alive).WithParkedMaxAge(0).Load(now)
		require.NoError(t, err)

		t.Run("it should list live sessions only", func(t *testing.T) {
			assert.Len(t, snapshot.Sessions, 1)
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
// ordinary directory adds a parked session to the city each time it
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
