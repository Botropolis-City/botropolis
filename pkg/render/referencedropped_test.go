package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 351, slice 4: a key a header no longer spells is still written
// once, in the reference, and the list vocabulary is one entry.
func TestTheReferenceHoldsWhatTheHeadersDropped(t *testing.T) {
	rows := func(want string) int {
		n := 0
		for _, k := range bindings {
			if strings.Contains(k.Key+" "+k.Action, want) {
				n++
			}
		}
		return n
	}

	t.Run("it should hold the list vocabulary in one entry", func(t *testing.T) {
		n := 0
		for _, k := range bindings {
			if k.Key == "in any list" {
				n++
			}
		}
		assert.Equal(t, 1, n)
	})

	for _, want := range []string{
		"PgUp",              // the reader's foot
		"replies",           // the reader's foot
		"restores",          // the journal's foot
		"same day",          // the long-range panel's foot
		"a month",           // the long-range panel's custom range
		"stand-up",          // City Hall's header
		"pauses",            // close-up's line
		"pins",              // close-up's line
		"takes the stepped", // the dive's hint
	} {
		t.Run("it should say "+want, func(t *testing.T) {
			assert.Positive(t, rows(want))
		})
	}
}
