package helpers_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 308: a city run under ZoneAt's zone reads a chosen hour, whatever
// the box's clock says.
func TestZoneAt(t *testing.T) {
	for _, utc := range []int{0, 5, 11, 12, 13, 23} {
		for _, hour := range []int{0, 10, 12, 22} {
			now := time.Date(2026, 10, 27, utc, 30, 0, 0, time.UTC)
			t.Run(fmt.Sprintf("when it is %02d:30 UTC, and %02d:00 is wanted", utc, hour), func(t *testing.T) {
				loc, err := time.LoadLocation(helpers.ZoneAt(hour, now))
				require.NoError(t, err)

				t.Run("it should read that hour", func(t *testing.T) {
					assert.Equal(t, hour, now.In(loc).Hour())
				})
			})
		}
	}
}
