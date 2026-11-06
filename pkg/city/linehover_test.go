package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 348, slice 4: a link of a group's line answers the pointer as its
// station did, with the card of the site it leads into, wherever the
// pointer is not on a building.
func TestLinkHover(t *testing.T) {
	other := func(id string) state.Session {
		s := session(id, mullet, state.Working)
		s.Group = "g1"
		return s
	}
	scene := func(t *testing.T, form city.LineForm) (*city.Scene, city.LineShape) {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(1600, 1000)
		s.SetSnapshot(withGroups(inOrder(in("a", "g1"), in("b", "g1"), loose("n"), other("d"), other("e"))))
		s.SetLineForm(form)
		shapes := s.Shapes()
		require.Len(t, shapes, 1)
		return s, shapes[0]
	}
	// clear is a point on a segment, as far along it as it takes to be
	// off every building.
	clear := func(t *testing.T, s *city.Scene, seg [2]city.Point) city.Point {
		t.Helper()
		for _, f := range []float64{0.5, 0.4, 0.6, 0.3, 0.7, 0.2, 0.8} {
			p := seg[0].Add(seg[1].Sub(seg[0]).Scale(f))
			if s.City().At(p).Building == nil {
				return p
			}
		}
		require.FailNow(t, "every point of the segment is on a building")
		return city.Point{}
	}
	point := func(s *city.Scene, p city.Point) city.Hit {
		s.PointerMove(s.Camera().WorldToScreen(p))
		return s.Hover()
	}

	t.Run("when the pointer is on a link", func(t *testing.T) {
		s, shape := scene(t, city.FormDistricts)
		hit := point(s, clear(t, s, shape.Links[0]))

		t.Run("it should hit the site the link leads into", func(t *testing.T) {
			require.NotNil(t, hit.Station)
			assert.Equal(t, shape.LinkSites[0].Site, hit.Station.Index)
		})

		t.Run("it should show that site's card", func(t *testing.T) {
			card, ok := s.Card()
			require.True(t, ok)
			assert.Equal(t, city.StationCard(s.City().Lines[0], shape.LinkSites[0].Site).Lines, card.Lines)
		})
	})

	t.Run("when the pointer is well off every link", func(t *testing.T) {
		s, shape := scene(t, city.FormDistricts)
		seg := shape.Links[0]
		far := clear(t, s, seg).Add(city.Point{X: 0, Y: 6 * city.LineHoverPixels / s.Camera().Zoom})
		hit := point(s, far)

		t.Run("it should hit no link", func(t *testing.T) {
			assert.Nil(t, hit.Station)
		})
	})

	t.Run("when the form is neighbourhoods, and the pointer is on an outline", func(t *testing.T) {
		s, shape := scene(t, city.FormNeighbourhoods)
		require.NotEmpty(t, shape.Outlines)
		// An outline runs along its lots' edges, so the pointer stands
		// just outside one, within the hover's reach and off the lot.
		var at city.Point
		found := false
		reach := city.LineHoverPixels / 2 / s.Camera().Zoom
		for _, seg := range shape.Outlines[0] {
			mid := seg[0].Add(seg[1].Sub(seg[0]).Scale(0.5))
			for _, off := range []city.Point{{X: reach}, {X: -reach}, {Y: reach}, {Y: -reach}} {
				if p := mid.Add(off); !found && s.City().At(p).Building == nil {
					at, found = p, true
				}
			}
		}
		require.True(t, found, "precondition: some outline edge faces open ground")
		hit := point(s, at)

		t.Run("it should hit the site the outline stands for", func(t *testing.T) {
			require.NotNil(t, hit.Station)
			assert.Equal(t, shape.OutlineSites[0], hit.Station.Index)
		})
	})

	t.Run("when the group lines are off", func(t *testing.T) {
		s, shape := scene(t, city.FormDistricts)
		s.SetGroupLines(false)
		hit := point(s, clear(t, s, shape.Links[0]))

		t.Run("it should hit no link", func(t *testing.T) {
			assert.Nil(t, hit.Station)
		})
	})
}
