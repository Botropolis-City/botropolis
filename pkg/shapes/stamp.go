package shapes

import "math"

// discRadii are the radii of the disc stamps, each √2 times the last, so
// a disc is drawn from a stamp shrunk by no more than √2 and its edge
// stays about a pixel soft.
var discRadii = func() []float64 {
	var r []float64
	for v := 0.25; v <= 160; v *= math.Sqrt2 {
		r = append(r, v)
	}
	return r
}()

// discStamp picks the stamp for a disc of radius r and the scale that
// makes it r: the smallest stamp no smaller than r, or the largest,
// scaled up.
func discStamp(r float64) (int, float64, bool) {
	if !(r > 0) {
		return 0, 0, false
	}
	for i, s := range discRadii {
		if s >= r-1e-9 {
			return i, r / s, true
		}
	}
	last := len(discRadii) - 1
	return last, r / discRadii[last], true
}
