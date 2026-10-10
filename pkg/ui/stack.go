package ui

import (
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// StackFittings is the curb where a flue passes through a flat roof:
// a low ring round the opening that the roofing turns up against.
// Bugs 46, 46a and 46b.
//
// The geometry, which is the whole of it. A cylinder meeting a
// horizontal plane intersects it in a circle, and a circle on the
// ground plane of a 2:1 projection is an ellipse as wide as the stack
// and half as tall. So the stack's visible bottom boundary is an arc
// bulging downward, and the sprite's own straight bottom edge is the
// one shape it cannot be.
//
// A ring of radius R round a stack of radius r is hidden only where it
// passes behind the stack, which is 2·asin(r/R) — at R = 1.32r that is
// 98°, leaving 262° visible. It wraps well past the horizontal diameter
// and its ends disappear behind the stack's silhouette rather than
// stopping in mid air, which is what made the first curb read as a
// detached crescent. None of that is computed here: the ring is drawn
// whole and the stack is drawn over it, so the occlusion falls out of
// the ordering.
type StackFittings struct {
	// OuterFoot is the ring's outer ellipse where it meets the roof,
	// and OuterTop the same ellipse at the top of the curb. The curb's
	// visible wall is the silhouette between them.
	OuterFoot city.Rect
	OuterTop  city.Rect
	// InnerTop is the ring's inner ellipse at the top of the curb. Its
	// near arc is the stack's new bottom line, and it is the stack's
	// own radius, so the curb meets the wall with no gap.
	InnerTop city.Rect
}

const (
	// StackPipeShare is how much of the chimney sprite's width is ink
	// **on the row the roof cuts it**. The sprite is 87 px wide in the
	// z2 cut and tapers all the way up, so the answer depends entirely
	// on which row is asked: 85 at the skirt, 75 a little above it, and
	// 63 at row 123, which is where the cut actually lands. The sprite
	// is 351 rows and the cut takes the top 123 of them, so the skirt
	// and the base ellipse are already far below the roof line.
	StackPipeShare = 63.0 / 87.0
	// StackCurbRatio is the curb's radius against the pipe's, and it
	// cannot be less than √2.
	//
	// The curb's near annulus has two jobs at once. It has to hide the
	// sprite's straight cut at the centre, which needs its inner rim at
	// or above the cut: h ≥ r/2. And it has to cover the sprite's
	// square-cut corners at x = ±r, which needs its outer rim at or
	// below them: h ≤ (R/2)·√(1−(r/R)²). Those two meet at R = √2·r and
	// are contradictory below it — at 1.32 the most a curb can cover is
	// 0.431r against the 0.5r it needs, so the corners show whatever
	// height is chosen. They are Aria's two white squares, and no
	// adjustment of the height would have found them.
	StackCurbRatio = 1.55
)

// CurbFloor is the least tall a curb may be and still hide the sprite's
// straight cut: its inner rim has to reach the cut line.
func CurbFloor(r float64) float64 { return r / 2 }

// CurbCeiling is the tallest a curb may be and still cover the sprite's
// square-cut corners: its outer rim has to reach down to them.
func CurbCeiling(r, outer float64) float64 {
	if outer <= r {
		return 0
	}
	return outer / 2 * math.Sqrt(1-(r/outer)*(r/outer))
}

// HiddenArc is how much of a ring of radius R round a stack of radius r
// passes behind the stack, in radians. Exported so the drawing's
// premise can be asserted rather than described.
func HiddenArc(r, outer float64) float64 {
	if outer < r || outer == 0 {
		return 0
	}
	return 2 * math.Asin(r/outer)
}

// ellipseAt is the bounding box of a circle of radius r about a centre,
// laid on the ground plane: as wide as the circle and half as tall.
func ellipseAt(cx, cy, r float64) city.Rect {
	return city.RectAt(cx-r, cy-r/2, 2*r, r)
}

// LayoutStackFittings places the curb on a stack drawn into the given
// screen rect, whose foot has been cut off at the roof plane.
//
// The curb's height is bounded at both ends by the geometry rather than
// chosen. Its top surface has to straddle the cut: if the inner rim
// sits below the cut the sprite's straight edge shows above it, and if
// the outer rim sits above the cut the straight edge shows below it.
// That puts the height between r/2 and R/2, and this takes the middle.
//
// There is no zoom term. A filled ring survives being small, which the
// hairline guy braces this replaced did not.
func LayoutStackFittings(stack city.Rect, roof float64) (StackFittings, bool) {
	if stack.Width() <= 0 || stack.Height() <= 0 {
		return StackFittings{}, false
	}
	cx := stack.Center().X
	r := stack.Width() / 2 * StackPipeShare
	outer := r * StackCurbRatio
	height := (CurbFloor(r) + CurbCeiling(r, outer)) / 2
	top := roof - height
	return StackFittings{
		OuterFoot: ellipseAt(cx, roof, outer),
		OuterTop:  ellipseAt(cx, top, outer),
		InnerTop:  ellipseAt(cx, top, r),
	}, true
}
