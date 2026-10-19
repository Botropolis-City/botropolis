package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 149: a daemon of another release is said once, as a chip that
// leads the strip so a narrow window cannot drop it.
func TestStripNotice(t *testing.T) {
	notice := "daemon is 0.2.2, this is 0.2.3: restart it to update"

	t.Run("when the summary carries a notice", func(t *testing.T) {
		chips := ui.StripChips(city.Summary{Working: 2, Notice: notice})

		t.Run("it should lead with it", func(t *testing.T) {
			assert.Equal(t, notice, chips[0].Text)
		})

		t.Run("it should tone it as wanting you", func(t *testing.T) {
			assert.Equal(t, ui.ToneNeedsYou, chips[0].Tone)
		})

		t.Run("it should give it no jump", func(t *testing.T) {
			assert.Empty(t, chips[0].State)
		})

		t.Run("it should show it once", func(t *testing.T) {
			count := 0
			for _, c := range chips {
				if c.Text == notice {
					count++
				}
			}
			assert.Equal(t, 1, count)
		})
	})

	t.Run("when the summary carries none", func(t *testing.T) {
		chips := ui.StripChips(city.Summary{Working: 2})

		t.Run("it should lead with the counts as before", func(t *testing.T) {
			assert.Equal(t, "2 working", chips[0].Text)
		})
	})
}
