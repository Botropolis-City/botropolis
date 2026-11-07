package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 351: the hold reaches through a list panel until it takes
// letters: its filter is typing, its name field has them, or a key is
// being recorded.
func TestListPanelsHoldCaps(t *testing.T) {
	for _, tc := range []struct {
		name    string
		resting capsHolder
		typing  capsHolder
	}{
		{"the clinic", &clinic{}, &clinic{filter: listFilter{on: true}}},
		{"City Hall's board", &board{}, &board{filter: listFilter{on: true}}},
		{"the journal", &journal{}, &journal{filter: listFilter{on: true}}},
		{"close-up", &closeUp{}, &closeUp{filter: listFilter{on: true}}},
		{"the group picker", &picker{}, &picker{naming: true}},
		{"the group picker's filter", &picker{}, &picker{filter: listFilter{on: true}}},
		{"the Keys panel", &keysPanel{}, &keysPanel{recording: true}},
	} {
		t.Run("when "+tc.name+" is up and takes no letters", func(t *testing.T) {
			t.Run("it should let the hold through", func(t *testing.T) {
				assert.True(t, tc.resting.holdsCaps())
			})
		})

		t.Run("when "+tc.name+" takes letters", func(t *testing.T) {
			t.Run("it should not", func(t *testing.T) {
				assert.False(t, tc.typing.holdsCaps())
			})
		})
	}
}
