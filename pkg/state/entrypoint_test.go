package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestSessionEntrypoint(t *testing.T) {
	for _, c := range []struct {
		when       string
		record     string
		transcript string
		want       string
	}{
		{"when the transcript names an entrypoint", "", "sdk-cli", "sdk-cli"},
		{"when only the session record names one", "sdk-ts", "", "sdk-ts"},
		{"when the record and the transcript disagree", "cli", "sdk-cli", "sdk-cli"},
	} {
		t.Run(c.when, func(t *testing.T) {
			r := record(sidA, claude.KindInteractive, claude.StatusIdle)
			r.Entrypoint = c.record
			tr := transcript(sidA, claude.TurnAwaitingUser)
			tr.Entrypoint = c.transcript
			session := build(t, []claude.SessionRecord{r}, []claude.Transcript{tr}, nil, alive)[0]

			t.Run("it should carry "+c.want+" on the session", func(t *testing.T) {
				assert.Equal(t, c.want, session.Entrypoint)
			})
		})
	}

	t.Run("when a parked transcript names a scripted entrypoint", func(t *testing.T) {
		tr := transcript(sidA, claude.TurnAwaitingUser)
		tr.Entrypoint = "sdk-cli"
		sessions := state.Build(state.Sources{Stopped: []claude.Transcript{tr}}, alive, now)
		require.Len(t, sessions, 1)

		t.Run("it should be scripted", func(t *testing.T) {
			assert.True(t, sessions[0].Scripted())
		})
	})

	t.Run("when a session names no entrypoint", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusIdle)},
			[]claude.Transcript{transcript(sidA, claude.TurnAwaitingUser)}, nil, alive)[0]

		t.Run("it should not be scripted", func(t *testing.T) {
			assert.False(t, session.Scripted())
		})
	})
}
