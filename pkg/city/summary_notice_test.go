package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 149: what the client says about its daemon reaches the strip.
func TestSummaryNotice(t *testing.T) {
	t.Run("when the snapshot carries a notice", func(t *testing.T) {
		snapshot := state.Snapshot{Notice: "daemon is 0.2.2, this is 0.2.3: restart it to update"}

		t.Run("it should be in the summary", func(t *testing.T) {
			assert.Equal(t, snapshot.Notice, city.Build(snapshot, city.NewLayout()).Summary().Notice)
		})
	})
}
