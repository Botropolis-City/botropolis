package city

// LineForm is how a group's line is drawn: as stations at its districts,
// as it always has been, or one of the three forms item 348 tries on
// frames for Aria to pick from.
type LineForm int

const (
	FormStations LineForm = iota
	// FormSessions runs building to building, inside a district and
	// between them.
	FormSessions
	// FormDistricts runs only between districts, joining the nearest pair
	// of their buildings.
	FormDistricts
	// FormNeighbourhoods outlines each neighbourhood and links one to
	// the next across districts.
	FormNeighbourhoods
)

func (f LineForm) String() string {
	switch f {
	case FormSessions:
		return "sessions"
	case FormDistricts:
		return "districts"
	case FormNeighbourhoods:
		return "neighbourhoods"
	}
	return "stations"
}

// LineShape is what a line draws in a form other than stations: segments
// in world space, and outlines, each a neighbourhood's edges.
type LineShape struct {
	Group    string
	Links    [][2]Point
	Outlines [][][2]Point
}

// Centre is the middle of a neighbourhood's block.
func (n Neighbourhood) Centre() Point {
	if len(n.Ground) == 0 {
		return Point{}
	}
	box := n.Ground[0]
	for _, r := range n.Ground[1:] {
		box.Min.X, box.Min.Y = min(box.Min.X, r.Min.X), min(box.Min.Y, r.Min.Y)
		box.Max.X, box.Max.Y = max(box.Max.X, r.Max.X), max(box.Max.Y, r.Max.Y)
	}
	return box.Center()
}

// Shape is a line drawn in a form, in the line's site order. A site with
// no session of the group, one only the coordinator's subagents worked
// in, or a pull request opened outside it, is skipped: the line does not
// go to districts (item 348).
func (c *City) Shape(l Line, form LineForm) LineShape {
	shape := LineShape{Group: l.Group.ID}
	if form == FormStations {
		return shape
	}
	var sites [][]*Building
	seen := map[*District]bool{}
	for _, st := range l.Stations {
		if st.Teammate != nil || st.By != nil || seen[st.District] {
			continue
		}
		seen[st.District] = true
		if bs := st.buildings(l.Group); len(bs) > 0 {
			sites = append(sites, bs)
		}
	}
	switch form {
	case FormSessions:
		var chain []Point
		for _, bs := range sites {
			for _, b := range bs {
				chain = append(chain, b.Rect.Center())
			}
		}
		for i := 1; i < len(chain); i++ {
			shape.Links = append(shape.Links, [2]Point{chain[i-1], chain[i]})
		}
	case FormDistricts:
		for i := 1; i < len(sites); i++ {
			shape.Links = append(shape.Links, nearestPair(sites[i-1], sites[i]))
		}
	case FormNeighbourhoods:
		var anchors []Point
		for _, bs := range sites {
			anchor := bs[0].Rect.Center()
			most := 0
			for _, n := range c.Neighbourhoods(c.DistrictOf(bs[0])) {
				if n.Group != l.Group {
					continue
				}
				shape.Outlines = append(shape.Outlines, n.Edges())
				if len(n.Buildings) > most {
					anchor, most = n.Centre(), len(n.Buildings)
				}
			}
			anchors = append(anchors, anchor)
		}
		for i := 1; i < len(anchors); i++ {
			shape.Links = append(shape.Links, [2]Point{anchors[i-1], anchors[i]})
		}
	}
	return shape
}

// nearestPair is the closest pair of buildings, one from each site, by
// their centres.
func nearestPair(a, b []*Building) [2]Point {
	var best [2]Point
	bestDist := -1.0
	for _, x := range a {
		for _, y := range b {
			p, q := x.Rect.Center(), y.Rect.Center()
			d := (p.X-q.X)*(p.X-q.X) + (p.Y-q.Y)*(p.Y-q.Y)
			if bestDist < 0 || d < bestDist {
				best, bestDist = [2]Point{p, q}, d
			}
		}
	}
	return best
}

// SetLineForm sets how the groups' lines are drawn.
func (s *Scene) SetLineForm(f LineForm) { s.lineForm = f }

// LineForm is how the groups' lines are drawn.
func (s *Scene) LineForm() LineForm { return s.lineForm }

// Shapes are the groups' lines in the scene's form, or none while the
// form is stations or the setting hides the lines. Unlike stations, a
// form draws a group in one district too.
func (s *Scene) Shapes() []LineShape {
	if s.linesHidden || s.lineForm == FormStations {
		return nil
	}
	var out []LineShape
	for _, l := range s.city.Lines {
		if shape := s.city.Shape(l, s.lineForm); len(shape.Links)+len(shape.Outlines) > 0 {
			out = append(out, shape)
		}
	}
	return out
}
