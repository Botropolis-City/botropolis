package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/appd"
)

// Item 240: a capture waits on the history's read and nothing else does,
// so it reads at full speed; an open city keeps to half a core.
func TestReadSpeed(t *testing.T) {
	for _, tc := range []struct {
		name               string
		screenshot, record string
		want               appd.Speed
	}{
		{"the city takes a screenshot", "frame.png", "", appd.FullSpeed},
		{"the city records", "", "frames", appd.FullSpeed},
		{"the city is opened to be used", "", "", appd.Paced},
	} {
		t.Run("when "+tc.name, func(t *testing.T) {
			t.Run("it should read its history at the speed for that", func(t *testing.T) {
				assert.Equal(t, tc.want, readSpeed(tc.screenshot, tc.record))
			})
		})
	}
}
