package city_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 108, Aria 2026-10-18: "We don't want driveways moving." A lot's
// driveway is chosen when it first has a road and kept from then on.
func TestDrivewaysAreKept(t *testing.T) {
	withRoads := func(pairs ...[2]string) state.Snapshot {
		return state.Snapshot{At: now, Sessions: sixDistricts(), Roads: roadsBetween(pairs...)}
	}
	driveway := func(c *city.City, name string) city.Cell {
		dw, _ := c.Driveway(districtNamed(c, name))
		return dw.Cell
	}

	t.Run("when a lot that has a driveway gains a road that would tip it to another side", func(t *testing.T) {
		// d's first road is to a, behind it, so its driveway is on its
		// back. A road to f, in front, ties the two sides, and a tie
		// would go to the side facing the camera.
		layout := city.NewLayout()
		first := city.Build(withRoads([2]string{"a", "d"}), layout)
		later := city.Build(withRoads([2]string{"a", "d"}, [2]string{"d", "f"}), layout)

		t.Run("it should keep its driveway", func(t *testing.T) {
			assert.Equal(t, driveway(first, "d"), driveway(later, "d"))
		})
	})

	t.Run("when the camera turns", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(1600, 1000)
		s.SetSnapshot(withRoads([2]string{"a", "d"}, [2]string{"b", "e"}))
		before := map[string]city.Cell{}
		for _, name := range []string{"a", "b", "d", "e"} {
			before[name] = driveway(s.City(), name)
		}
		s.Turn()
		after := map[string]city.Cell{}
		for _, name := range []string{"a", "b", "d", "e"} {
			after[name] = driveway(s.City(), name)
		}

		t.Run("it should keep every driveway that has been chosen", func(t *testing.T) {
			assert.Equal(t, before, after)
		})
	})

	t.Run("when the map is opened again", func(t *testing.T) {
		layout := city.NewLayout()
		first := city.Build(withRoads([2]string{"a", "d"}), layout)
		path := filepath.Join(t.TempDir(), "layout.json")
		require.NoError(t, layout.Save(path))
		reopened, err := city.LoadLayout(path)
		require.NoError(t, err)
		later := city.Build(withRoads([2]string{"a", "d"}, [2]string{"d", "f"}), reopened)

		t.Run("it should keep the driveway it chose before", func(t *testing.T) {
			assert.Equal(t, driveway(first, "d"), driveway(later, "d"))
		})
	})
}
