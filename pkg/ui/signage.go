package ui

import (
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// Sign is a name painted on a building at the map's own scale, the way
// a company's name sits on its office block: never a floating plate.
// At is the text's origin on screen; Vertical means it runs up the
// side, rotated a quarter turn anticlockwise about At.
type Sign struct {
	Text     string
	At       city.Point
	Scale    float64
	Vertical bool
	// Lean shears the text vertically about At, in screen pixels of
	// rise per pixel across, so paint on a board that stands in the
	// world lies in the board's plane rather than square to the screen.
	// Zero is flat against the screen, which is every sign but a
	// signage board's.
	Lean float64
}

const (
	// SignFill is how much of the face a sign may take, so it reads as
	// painted on rather than pasted over the edge.
	SignFill = 0.8
	// MinSignPx is the smallest text height still legible; below it
	// the sign is left off and the hover plate carries the name.
	MinSignPx = 7.0
	// BoardLean is the shear of a board facing along the world's x
	// axis, in this 2:1 projection: one pixel down for every two
	// across. A board facing the other way leans the other sign.
	BoardLean = 0.5
)

// LayoutPlaque fits a name on a board that stands in the world — a
// gantry's panel, a post-mounted board, a plaque on a wall — centred on
// the face and leaning into the board's plane. It declines when the
// board is too small to read, and the hover plate carries the name
// instead, exactly as a tower's sign does.
func LayoutPlaque(name string, face city.Rect, lean float64, measure Measure) (Sign, bool) {
	w, h := measure(name, Small)
	if name == "" || w <= 0 || h <= 0 {
		return Sign{}, false
	}
	scale := fit(face.Width(), face.Height(), w, h)
	if h*scale < MinSignPx {
		return Sign{}, false
	}
	return Sign{
		Text:  name,
		Scale: scale,
		Lean:  lean,
		At: city.Point{
			X: face.Center().X - w*scale/2,
			Y: face.Center().Y - h*scale/2,
		},
	}, true
}

// LayoutSign fits a name on a tower, across the face or up the side,
// whichever leaves it largest (across the face when equal), and hangs
// nothing when even that is too small to read.
func LayoutSign(name string, face, side city.Rect, measure Measure) (Sign, bool) {
	w, h := measure(name, Small)
	if w <= 0 || h <= 0 {
		return Sign{}, false
	}
	across := math.Min(face.Width()*SignFill/w, face.Height()*SignFill/h)
	up := math.Min(side.Height()*SignFill/w, side.Width()*SignFill/h)
	switch {
	case across >= up && h*across >= MinSignPx:
		return Sign{Text: name, Scale: across, At: city.Point{
			X: face.Center().X - w*across/2,
			Y: face.Center().Y - h*across/2,
		}}, true
	case h*up >= MinSignPx:
		return Sign{Text: name, Scale: up, Vertical: true, At: city.Point{
			X: side.Center().X - h*up/2,
			Y: side.Center().Y + w*up/2,
		}}, true
	}
	return Sign{}, false
}

// BuildingSign is a session's title on its building, the way a firm's
// name goes on its office: a fascia over the door when it is short
// enough to read there, run up the side of a tall building when it is
// not, and a rooftop billboard of two ellipsised lines when it is too
// long for either. Nothing goes on below MinSignPx — the hover plate
// carries the name then, as it does for a tower.
type BuildingSign struct {
	Lines []Sign
	// Panel is the billboard's board and Posts the two legs holding it
	// up; both are zero for a fascia or a sign up the side.
	Panel city.Rect
	Posts [2]city.Rect
}

// Billboard reports whether the sign is a rooftop board rather than
// paint on the building itself.
func (s BuildingSign) Billboard() bool { return s.Panel.Area() > 0 }

const (
	// BillboardLines is how many lines a rooftop board carries.
	BillboardLines = 2
	// billboardLead is the line pitch as a multiple of the text height.
	billboardLead = 1.25
	// billboardPad is the board's margin round its text, in text heights.
	billboardPad = 0.35
	// postWidth is a billboard post's width as a fraction of the board.
	postWidth = 0.06
)

// LayoutBuildingSign fits a session's title on its building. fascia is
// the band over the door on the front face, side the strip up the
// building's flank, and roof the space above the roofline a billboard
// may stand in.
func LayoutBuildingSign(title string, fascia, side, roof city.Rect, measure Measure) (BuildingSign, bool) {
	w, h := measure(title, Small)
	if title == "" || w <= 0 || h <= 0 {
		return BuildingSign{}, false
	}
	if across := fit(fascia.Width(), fascia.Height(), w, h); h*across >= MinSignPx {
		return BuildingSign{Lines: []Sign{{Text: title, Scale: across, At: city.Point{
			X: fascia.Center().X - w*across/2,
			Y: fascia.Center().Y - h*across/2,
		}}}}, true
	}
	if up := fit(side.Height(), side.Width(), w, h); h*up >= MinSignPx {
		return BuildingSign{Lines: []Sign{{Text: title, Scale: up, Vertical: true, At: city.Point{
			X: side.Center().X - h*up/2,
			Y: side.Center().Y + w*up/2,
		}}}}, true
	}
	return billboard(title, roof, h, measure)
}

// fit is the largest scale that keeps w by h inside a box, leaving
// SignFill of it for the text.
func fit(boxW, boxH, w, h float64) float64 {
	return math.Min(boxW*SignFill/w, boxH*SignFill/h)
}

// billboard puts the title on a rooftop board: two lines split at the
// space nearest the middle, each ellipsised to the board, on a panel
// held up by two posts.
func billboard(title string, roof city.Rect, h float64, measure Measure) (BuildingSign, bool) {
	if roof.Width() <= 0 || roof.Height() <= 0 {
		return BuildingSign{}, false
	}
	scale := roof.Height() * SignFill / (h * (BillboardLines*billboardLead + 2*billboardPad))
	if h*scale < MinSignPx {
		return BuildingSign{}, false
	}
	inner := roof.Width()*SignFill - 2*billboardPad*h*scale
	if inner <= 0 {
		return BuildingSign{}, false
	}
	var lines []string
	for _, part := range splitTwo(title) {
		lines = append(lines, ellipsise(part, inner/scale, measure))
	}
	widest := 0.0
	for _, l := range lines {
		lw, _ := measure(l, Small)
		widest = math.Max(widest, lw)
	}
	pad := billboardPad * h * scale
	panel := centredRect(roof.Center(), widest*scale+2*pad, float64(len(lines))*billboardLead*h*scale+2*pad)
	sign := BuildingSign{Panel: panel}
	for i, l := range lines {
		lw, _ := measure(l, Small)
		sign.Lines = append(sign.Lines, Sign{Text: l, Scale: scale, At: city.Point{
			X: panel.Center().X - lw*scale/2,
			Y: panel.Min.Y + pad + float64(i)*billboardLead*h*scale,
		}})
	}
	leg := panel.Width() * postWidth
	for i, x := range [2]float64{panel.Min.X + panel.Width()*0.25, panel.Min.X + panel.Width()*0.75} {
		sign.Posts[i] = city.Rect{
			Min: city.Point{X: x - leg/2, Y: panel.Max.Y},
			Max: city.Point{X: x + leg/2, Y: roof.Max.Y},
		}
	}
	return sign, true
}

func centredRect(at city.Point, w, h float64) city.Rect {
	return city.RectAt(at.X-w/2, at.Y-h/2, w, h)
}

// splitTwo breaks a title into two lines at the space nearest its
// middle, or leaves it as one line when it has no space to break at.
func splitTwo(title string) []string {
	runes := []rune(title)
	best := -1
	for i, r := range runes {
		if r != ' ' {
			continue
		}
		if best < 0 || abs(i-len(runes)/2) < abs(best-len(runes)/2) {
			best = i
		}
	}
	if best <= 0 || best >= len(runes)-1 {
		return []string{title}
	}
	return []string{string(runes[:best]), string(runes[best+1:])}
}

// ellipsise cuts a line to a width in unscaled text units, ending it
// with an ellipsis when anything had to go.
func ellipsise(line string, width float64, measure Measure) string {
	if w, _ := measure(line, Small); w <= width {
		return line
	}
	runes := []rune(line)
	for n := len(runes) - 1; n > 0; n-- {
		cut := string(runes[:n]) + "…"
		if w, _ := measure(cut, Small); w <= width {
			return cut
		}
	}
	return "…"
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
