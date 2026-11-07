package ui

import "botropolis.city/botropolis/pkg/city"

const (
	minimapWidth  = 160.0
	minimapHeight = 96.0
)

// LayoutMinimap is the minimap's box in the bottom-right corner above the
// room reserved along the bottom, or false when the window is too narrow to spare the room.
func LayoutMinimap(th Theme, width, height, reserved float64) (city.Rect, bool) {
	w, h := th.Px(minimapWidth), th.Px(minimapHeight)
	if width < 3*w {
		return city.Rect{}, false
	}
	margin := 2 * th.Grid()
	return city.RectAt(width-margin-w, height-reserved-margin-h, w, h), true
}
