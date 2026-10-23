package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 205: Build sorts the live and the stopped sessions apart, so a
// loader can keep the stopped half, and merges them, which must order
// them as the one stable sort of both, live first, always did.
func TestBuildOrder(t *testing.T) {
	t.Run("when a live and a stopped session share a directory and a start", func(t *testing.T) {
		const stoppedSid = "eeeeeeee-0000-4205-8000-000000000002"
		live := record(sidA, claude.KindInteractive, claude.StatusIdle)
		stopped := transcript(stoppedSid, claude.TurnAwaitingUser)
		stopped.CWD, stopped.FirstAt = live.CWD, started
		sessions := state.Build(state.Sources{Records: []claude.SessionRecord{live}, Stopped: []claude.Transcript{stopped}}, alive, now)
		var ids []string
		for _, s := range sessions {
			ids = append(ids, s.ID)
		}

		t.Run("it should put the live one first", func(t *testing.T) {
			assert.Equal(t, []string{sidA, stoppedSid}, ids)
		})
	})
}
