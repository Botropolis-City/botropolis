package city

// Skyline. Item 89.
//
// The isometric map zooms out no further than where a building cell
// still draws FloorBelow pixels wide, so at its furthest the buildings,
// their signs, their drones and their work all still read. Skyline, on
// f, is the whole city when it reads at that floor, and otherwise the
// floor centred on the city. A fresh map opens at Skyline.

// FloorBelow is the drawn width of a building cell, in pixels, below
// which the isometric map does not zoom out.
const FloorBelow = 14.0

// FloorZoom is the furthest a projection zooms out: where a building
// cell draws FloorBelow wide on the isometric map. The top-down map
// keeps the floors it had.
func FloorZoom(p Projection) float64 {
	if p != Isometric {
		return 0
	}
	c := NewCamera()
	c.Projection = p
	w := c.Bounds(RectAt(0, 0, CellSize, CellSize)).Width()
	if w <= 0 {
		return 0
	}
	return FloorBelow / w
}

// FitZoom is the zoom that would show the whole city, without moving the
// camera or holding it to the floor.
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

// Skyline frames the city: whole if it reads at the floor, otherwise the
// floor, centred on it.
func (s *Scene) Skyline() { s.Fit() }

// floored holds the camera to the floor, keeping the middle of the view
// where it is.
func (s *Scene) floored() {
	floor := FloorZoom(s.camera.Projection)
	if s.camera.Zoom >= floor {
		return
	}
	in := s.Insets()
	centre := Point{X: in.Left + (s.width-in.Left-in.Right)/2, Y: in.Top + (s.height-in.Top-in.Bottom)/2}
	s.camera.SetZoomAt(centre, floor)
}
