package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// beamTo is the beam to the named session, or nil.
func beamTo(s *city.Scene, id string) *city.Beam {
	for _, b := range s.City().Beams() {
		if b.Building.Session.ID == id {
			return &b
		}
	}
	return nil
}

// Item 347: a packed session kept its line from every tower it ever
// called, so a depot of ninety crates had lines across the whole empty
// south of the map. The lines run to the buildings on the map unless
// the setting asks for the crates too.
func TestTowerLines(t *testing.T) {
	live := caller("live", cinders, map[string]int{"atlassian": 5})
	packed := caller("packed", botropolis, map[string]int{"atlassian": 3})
	packed.Packed, packed.PackedAt = true, now.Add(-time.Hour)

	t.Run("when a packed session called a server, and lines run to buildings only", func(t *testing.T) {
		s := served(t, live, packed)
		require.False(t, s.DepotBeams(), "precondition: buildings only is the default")
		crate := buildingOf(t, s, "packed")
		require.True(t, crate.Packed, "precondition: the session is a crate")

		t.Run("it should run no beam to its crate in the depot", func(t *testing.T) {
			assert.Nil(t, beamTo(s, "packed"))
		})

		t.Run("it should still run the beam to the live session that called it", func(t *testing.T) {
			assert.NotNil(t, beamTo(s, "live"))
		})

		t.Run("it should offer nothing to point at where the crate's line would be", func(t *testing.T) {
			from, to := towerNamed(t, s, "atlassian").Rect.Center(), crate.Rect.Center()
			mid := city.Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
			assert.Nil(t, s.City().Near(mid, 1).Beam)
		})

		t.Run("and the setting changes with the city already built", func(t *testing.T) {
			s.SetDepotBeams(true)

			t.Run("it should run a beam to the crate without a new snapshot", func(t *testing.T) {
				assert.NotNil(t, beamTo(s, "packed"))
			})
		})
	})

	t.Run("when lines run to all sessions before the city is built", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.SetDepotBeams(true)
		s = servedOn(t, s, live, packed)

		t.Run("it should run a beam to the crate", func(t *testing.T) {
			assert.NotNil(t, beamTo(s, "packed"))
		})
	})
}

// Item 347: the Connections lens says what its lines cover, and switches
// the depot's in and out from its own panel.
func TestLensPanelTowerLines(t *testing.T) {
	live := caller("live", cinders, map[string]int{"atlassian": 5, "playwright": 1})
	other := caller("other", botropolis, map[string]int{"atlassian": 2})
	packed := caller("packed", botropolis, map[string]int{"atlassian": 3})
	packed.Packed, packed.PackedAt = true, now.Add(-time.Hour)

	t.Run("when the Connections lens is on the servers view, and lines run to buildings only", func(t *testing.T) {
		s := served(t, live, other, packed)
		s.SetView(city.ViewServers)
		p := s.LensPanel()

		t.Run("the aggregate should count the lines, towers and sessions on the map", func(t *testing.T) {
			assert.Equal(t, "3 lines from 2 towers to 2 sessions on the map", p.Aggregate)
		})

		t.Run("the switch should offer the depot", func(t *testing.T) {
			assert.Equal(t, "show lines to the depot too", p.Switch)
		})
	})

	t.Run("when the Connections lens is on the servers view, and lines run to all sessions", func(t *testing.T) {
		s := served(t, live, other, packed)
		s.SetView(city.ViewServers)
		s.SetDepotBeams(true)
		p := s.LensPanel()

		t.Run("the aggregate should count the crates too", func(t *testing.T) {
			assert.Equal(t, "4 lines from 2 towers to 2 sessions and 1 crate", p.Aggregate)
		})

		t.Run("the switch should offer buildings only", func(t *testing.T) {
			assert.Equal(t, "lines to buildings only", p.Switch)
		})
	})

	t.Run("when the lens is on the traffic view", func(t *testing.T) {
		s := served(t, live, other, packed)
		s.SetView(city.ViewTraffic)
		p := s.LensPanel()

		t.Run("the aggregate should still count the roads", func(t *testing.T) {
			assert.Equal(t, "0 roads between projects", p.Aggregate)
		})

		t.Run("there should be no switch", func(t *testing.T) {
			assert.Empty(t, p.Switch)
		})
	})
}
