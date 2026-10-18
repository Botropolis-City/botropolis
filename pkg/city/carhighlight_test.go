package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 108: pointing at a car lights its path, driveway to driveway, and
// recedes everything but the two lots it runs between.
func TestCarHighlight(t *testing.T) {
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(1600, 1000)
	s.SetSnapshot(state.Snapshot{At: now, Sessions: sixDistricts(), Roads: roadsBetween([2]string{"a", "b"}, [2]string{"c", "f"})})
	c := s.City()
	ab, ok := streetOf(c, "a", "b")
	require.True(t, ok, "precondition: the road is routed")
	cf, ok := streetOf(c, "c", "f")
	require.True(t, ok, "precondition: the other road is routed")
	ours := map[*city.District]bool{districtNamed(c, "a"): true, districtNamed(c, "b"): true}

	t.Run("when the pointer is on a car", func(t *testing.T) {
		s.SetHover(city.Hit{Car: &city.CarHit{Road: ab.Road, From: "a", To: "b"}})
		h := s.Highlight()

		t.Run("it should light that car's road", func(t *testing.T) {
			assert.True(t, h.Active && h.HasRoad(ab.Road))
		})

		t.Run("it should light no other road", func(t *testing.T) {
			assert.False(t, h.HasRoad(cf.Road))
		})

		t.Run("it should light every building in its two districts and none elsewhere", func(t *testing.T) {
			var wrong []string
			for _, b := range c.Buildings() {
				if h.HasBuilding(b) != ours[c.DistrictOf(b)] {
					wrong = append(wrong, b.Session.ID)
				}
			}
			assert.Empty(t, wrong)
		})

		t.Run("it should keep its two lots and recede every other", func(t *testing.T) {
			var wrong []string
			for _, d := range c.Districts {
				if h.Recedes(d) == ours[d] {
					wrong = append(wrong, d.Name)
				}
			}
			assert.Empty(t, wrong)
		})

		t.Run("it should recede every other road's ground", func(t *testing.T) {
			assert.True(t, h.RecedesGround())
		})
	})

	t.Run("when the pointer is on a building", func(t *testing.T) {
		s.SetHover(city.Hit{Building: c.Buildings()[0]})
		h := s.Highlight()

		t.Run("it should recede no lot", func(t *testing.T) {
			var receded []string
			for _, d := range c.Districts {
				if h.Recedes(d) {
					receded = append(receded, d.Name)
				}
			}
			assert.Empty(t, receded)
		})

		t.Run("it should recede no road's ground", func(t *testing.T) {
			assert.False(t, h.RecedesGround())
		})
	})
}
