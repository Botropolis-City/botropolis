package ui

import "botropolis.city/botropolis/pkg/city"

// Monument is the sign that stands outside an office plaza, built to
// the grammar of Aria's references in docs/references: coursed stone
// piers either side of a recessed panel, a cornice oversailing them,
// and a base course at the ground. Bugs 44 and 45.
//
// It faces the viewer rather than lying in the world's plane. Type
// sheared into a 2:1 projection is hard to read at any size, so the
// face is a rectangle in screen space and the copy sits level. What
// stops that reading as a decal is not the angle but the depth: the
// piers keep their returns, the cornice keeps a visible underside, and
// the whole thing casts a contact shadow onto the ground it stands on.
//
// Two rules define the object and are asserted rather than commented. A
// monument is low and landscape, and it is a mass sitting on the earth
// — a panel floating clear of its base is a pylon sign, which is the
// failure mode.
type Monument struct {
	// Shadow is the contact patch on the ground at its foot, which is
	// what the Oak Hollow reference has that keeps it from hovering.
	Shadow city.Rect
	// The masonry, ground upwards. Base is the course that meets the
	// earth and is the widest element; Left and Right are the piers;
	// Cornice oversails them and Soffit is its underside, which stays
	// visible and darker because that is the strongest single cue that
	// a thing is solid.
	Base    city.Rect
	Left    city.Rect
	Right   city.Rect
	Cornice city.Rect
	Soffit  city.Rect
	// Panel is the smooth field recessed between the piers, and Reveal
	// is the one-pixel shadow along its top and inside edge that sells
	// the recess. Flush reads as printed; recessed reads as built.
	Panel  city.Rect
	Reveal city.Rect
	// Courses are the score lines that make the stone read as coursed.
	// At this size that is a few lines with a value shift, not a
	// texture and not noise.
	Courses []city.Rect
	// Copy is the project's name, level on the panel.
	Copy Sign
}

// Whole is the sign's full extent, base to cornice.
func (m Monument) Whole() city.Rect {
	return city.Rect{
		Min: city.Point{X: m.Base.Min.X, Y: m.Cornice.Min.Y},
		Max: city.Point{X: m.Base.Max.X, Y: m.Base.Max.Y},
	}
}

const (
	// MonumentAspect is width against height. Real monument signs run
	// 4-6 ft by 8-12 ft; 2:1 is the middle of that.
	MonumentAspect = 2.0
	// MonumentGround is how much of the sign's width has to meet the
	// ground. Codes commonly require 40%, and it is the rule that makes
	// this a monument rather than a board on posts.
	MonumentGround = 0.4

	// The elements' shares of the sign's height, ground upwards.
	monumentBaseShare    = 0.17
	monumentCorniceShare = 0.13
	monumentSoffitShare  = 0.04
	// The elements' shares of its width. The body is inset from the
	// base course so the base reads as wider than what stands on it,
	// and the cornice oversails the body on every side.
	monumentBodyInset = 0.06
	monumentOversail  = 0.035
	monumentPier      = 0.15
	// monumentReveal is the recess's shadow, and monumentCourse the
	// score lines' thickness, both as shares of the height.
	monumentReveal = 0.035
	monumentCourse = 0.022
	// MonumentCopyAbove is the copy's margin inside the panel.
	MonumentCopyAbove = 0.18
)

// MonumentFootprint is how wide the ground a sign of this width covers.
// It is wider than the sign, because the contact patch oversails the
// base course, and the siting has to pull back by what the sign really
// covers rather than by what it nominally is.
func MonumentFootprint(width float64) float64 {
	return width * (1 + 2*monumentOversail)
}

// MonumentWidest is the widest panel a base of this width could carry
// without breaking the 40% rule. It is the limit, not the design: a
// sign built to it has a base two fifths of its width, which is a board
// on a plinth and is the shape the rule exists to keep out.
func MonumentWidest(base float64) float64 {
	return base / MonumentGround
}

// MonumentOnGround is the share of a sign's width that meets the
// ground, which has to be at least MonumentGround.
func MonumentOnGround(base, width float64) float64 {
	if width <= 0 {
		return 0
	}
	return base / width
}

// LayoutMonument builds the sign standing on a ground point, facing the
// viewer, width wide. It declines when the copy would be too small to
// read, exactly as a tower's sign does, and the hover plate carries the
// name instead.
func LayoutMonument(name string, foot city.Point, width float64, measure Measure) (Monument, bool) {
	if name == "" || width <= 0 {
		return Monument{}, false
	}
	height := width / MonumentAspect
	left, top := foot.X-width/2, foot.Y-height
	baseH := height * monumentBaseShare
	cornH := height * monumentCorniceShare
	soffH := height * monumentSoffitShare

	inset := width * monumentBodyInset
	bodyW := width - 2*inset
	over := width * monumentOversail
	pierW := width * monumentPier

	m := Monument{
		Base:    city.RectAt(left, foot.Y-baseH, width, baseH),
		Cornice: city.RectAt(left+inset-over, top, bodyW+2*over, cornH),
	}
	m.Soffit = city.RectAt(m.Cornice.Min.X, m.Cornice.Max.Y, m.Cornice.Width(), soffH)

	bodyTop, bodyBottom := m.Soffit.Max.Y, m.Base.Min.Y
	m.Left = city.RectAt(left+inset, bodyTop, pierW, bodyBottom-bodyTop)
	m.Right = city.RectAt(left+inset+bodyW-pierW, bodyTop, pierW, bodyBottom-bodyTop)
	m.Panel = city.Rect{
		Min: city.Point{X: m.Left.Max.X, Y: bodyTop},
		Max: city.Point{X: m.Right.Min.X, Y: bodyBottom},
	}
	m.Reveal = city.RectAt(m.Panel.Min.X, m.Panel.Min.Y, m.Panel.Width(), height*monumentReveal)

	// The contact patch. It sits mostly *below* the foot, on the ground
	// in front of the base, because the base is drawn over it and a
	// shadow hidden behind the thing casting it is no shadow at all —
	// which is what the first cut of this was.
	shadowH := baseH * 1.7
	patch := MonumentFootprint(width)
	m.Shadow = city.RectAt(foot.X-patch/2, foot.Y-shadowH*0.32, patch, shadowH)

	m.Courses = courseLines(m, height)

	pad := m.Panel.Height() * MonumentCopyAbove
	field := city.Rect{
		Min: city.Point{X: m.Panel.Min.X + pad, Y: m.Panel.Min.Y + pad},
		Max: city.Point{X: m.Panel.Max.X - pad, Y: m.Panel.Max.Y - pad},
	}
	// Level, not leaning: the whole point of turning the face to the
	// viewer is that the type stops being sheared.
	sign, ok := LayoutPlaque(name, field, 0, measure)
	if !ok {
		return Monument{}, false
	}
	m.Copy = sign
	return m, true
}

// courseLines scores the piers and the base so the stone reads as
// coursed rather than poured.
func courseLines(m Monument, height float64) []city.Rect {
	thick := height * monumentCourse
	var out []city.Rect
	for _, pier := range [2]city.Rect{m.Left, m.Right} {
		for i := 1; i <= 2; i++ {
			y := pier.Min.Y + pier.Height()*float64(i)/3
			out = append(out, city.RectAt(pier.Min.X, y, pier.Width(), thick))
		}
	}
	out = append(out, city.RectAt(m.Base.Min.X, m.Base.Min.Y+m.Base.Height()/2, m.Base.Width(), thick))
	return out
}
