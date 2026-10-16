package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 84: a screenshot waits for its script. Draws can outrun updates
// on a real X server, and the frame was taken before the last keys were
// pressed.
func TestShotReady(t *testing.T) {
	for _, c := range []struct {
		scenario string
		script   []string
		frames   int
		ready    bool
	}{
		{"when keys remain to be pressed, however many frames were drawn", []string{"enter"}, 9, false},
		{"when the script is done and the frame has settled", nil, 3, true},
		{"when the script is done and the frame has not settled", nil, 2, false},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			g := &Game{screenshot: "frame.png", script: c.script, shotFrames: c.frames}

			t.Run("it should say whether to take the frame", func(t *testing.T) {
				assert.Equal(t, c.ready, g.shotReady())
			})
		})
	}
}
