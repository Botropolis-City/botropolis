package city

import (
	"math"

	"botropolis.city/botropolis/pkg/state"
)

// Semantic zoom. Item 89.
//
// At fit, and whenever a building would draw too small to read, a
// district draws as a badge: its sign alone, with how many sessions it
// has and how many need you. Past fit the threshold is the drawn width
// of one building cell, so it is the same however many districts the
// city has. Selecting a badge flies to its district.

// BadgeBelow is the drawn width of a building cell, in pixels, under
// which buildings give way to badges.
const BadgeBelow = 14.0

// Badge is a district drawn as its sign.
type Badge struct {
	District *District
	Sign     DistrictSign
	Sessions int
	NeedsYou int
	At       Point
}

// BadgeZoom is the zoom at which a building cell draws BadgeBelow wide
// in a projection.
func BadgeZoom(p Projection) float64 {
	c := NewCamera()
	c.Projection = p
	w := c.Bounds(RectAt(0, 0, CellSize, CellSize)).Width()
	if w <= 0 {
		return 0
	}
	return BadgeBelow / w
}

// FitZoom is the zoom the fit would choose, without moving the camera.
func (s *Scene) FitZoom() float64 {
	if len(s.city.Districts) == 0 {
		return 0
	}
	c := *s.camera
	c.FitWithInsets(s.city.Extent(), s.width, s.height, s.Insets())
	if c.Zoom < LabelZoom {
		c.FitWithInsets(s.city.Extent(), s.width, s.height, s.chromeInsets())
	}
	return c.Zoom
}

// Badged reports whether districts draw as badges: at fit or further
// out, or past fit when a building would draw too small to read.
func (s *Scene) Badged() bool {
	if len(s.city.Districts) == 0 {
		return false
	}
	zoom := s.camera.Zoom
	return zoom <= s.FitZoom()*(1+1e-9) || zoom < BadgeZoom(s.camera.Projection)
}

// Badges is every district as a badge, when they are drawn as badges.
func (s *Scene) Badges() []Badge {
	if !s.Badged() {
		return nil
	}
	var out []Badge
	for _, d := range s.city.Districts {
		if d.Depot {
			continue
		}
		b := Badge{District: d, Sign: d.Sign(), At: d.Rect.Center(), Sessions: len(d.Buildings)}
		for _, bd := range d.Buildings {
			if bd.Session.State == state.NeedsYou {
				b.NeedsYou++
			}
		}
		out = append(out, b)
	}
	return out
}

// badgeAt is the district whose badge stands over a world point, when
// badges are drawn.
func (s *Scene) badgeAt(world Point) *District {
	if !s.Badged() {
		return nil
	}
	for _, d := range s.city.Districts {
		if !d.Depot && d.Rect.Contains(world) {
			return d
		}
	}
	return nil
}

// flyToDistrict centres a district at a zoom where its buildings draw.
func (s *Scene) flyToDistrict(d *District) {
	zoom := math.Min(MaxZoom, math.Max(s.camera.Zoom, math.Max(BadgeZoom(s.camera.Projection), s.FitZoom())*1.25))
	s.touched = true
	s.zoomTarget = 0
	s.camera.Zoom = zoom
	in := s.Insets()
	target := Point{X: in.Left + (s.width-in.Left-in.Right)/2, Y: in.Top + (s.height-in.Top-in.Bottom)/2}
	s.camera.Offset = target.Scale(1 / s.camera.Zoom).Sub(s.camera.Project(d.Rect.Center()))
}
