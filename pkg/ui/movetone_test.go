package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

func TestMoveTone(t *testing.T) {
	cases := []struct {
		move state.Move
		tone ui.Tone
	}{
		{state.MoveYours, ui.ToneNeedsYou},
		{state.MoveAgents, ui.ToneWorking},
		{state.MoveTheirs, ui.ToneWaiting},
		{state.MoveNobody, ui.ToneMerged},
	}
	for _, c := range cases {
		t.Run("when the move is "+string(c.move), func(t *testing.T) {
			t.Run("it should be drawn in the "+c.tone.String()+" tone", func(t *testing.T) {
				assert.Equal(t, c.tone, ui.MoveTone(c.move))
			})
		})
	}
}
