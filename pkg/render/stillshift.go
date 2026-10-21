package render

import (
	"image"
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// A pan by whole pixels moves every still drawable by the same whole
// pixels and changes nothing else about it, so the standing city is
// shifted rather than composed again, and only the strips the move
// uncovers are composed (item 193). What a drawable paints is hashed
// relative to the camera's origin for that reason: a whole-pixel pan
// leaves every hash as it was.

// cameraOrigin is where the camera puts the world's origin on screen,
// which every world position is drawn relative to.
func cameraOrigin(cam *city.Camera) city.Point {
	return cam.Offset.Scale(cam.Zoom)
}

// wholeShift is how far, in whole pixels, the frame moved from one
// origin to another, and whether a layer can be shifted that far: the
// move must be whole pixels to within rounding, and leave some of the
// layer on the frame.
func wholeShift(from, to city.Point, frame image.Rectangle) (image.Point, bool) {
	dx, dy := to.X-from.X, to.Y-from.Y
	kx, ky := math.Round(dx), math.Round(dy)
	if math.Abs(dx-kx) > 1e-6 || math.Abs(dy-ky) > 1e-6 {
		return image.Point{}, false
	}
	k := image.Pt(int(kx), int(ky))
	if abs(k.X) >= frame.Dx() || abs(k.Y) >= frame.Dy() {
		return image.Point{}, false
	}
	return k, true
}

// exposed is the part of the frame a layer shifted by k no longer
// covers: a column on the side it moved from, and a row on the side it
// moved from beside it.
func exposed(frame image.Rectangle, k image.Point) []image.Rectangle {
	var out []image.Rectangle
	x0, x1 := frame.Min.X, frame.Max.X
	switch {
	case k.X > 0:
		out = append(out, image.Rect(frame.Min.X, frame.Min.Y, frame.Min.X+k.X, frame.Max.Y))
		x0 = frame.Min.X + k.X
	case k.X < 0:
		out = append(out, image.Rect(frame.Max.X+k.X, frame.Min.Y, frame.Max.X, frame.Max.Y))
		x1 = frame.Max.X + k.X
	}
	switch {
	case k.Y > 0:
		out = append(out, image.Rect(x0, frame.Min.Y, x1, frame.Min.Y+k.Y))
	case k.Y < 0:
		out = append(out, image.Rect(x0, frame.Max.Y+k.Y, x1, frame.Max.Y))
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// relQ is a screen coordinate relative to the camera's origin, in 256ths
// of a pixel: what a hash holds of a position, so a whole-pixel pan does
// not change it and rounding in the last bits does not either.
func relQ(v, origin float64) uint64 {
	return uint64(int64(math.Round((v - origin) * 256)))
}
