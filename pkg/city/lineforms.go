package city

// LineForm is how a group's line is drawn, the group_line_form setting:
// between its districts, the default, as its neighbourhoods' outlines
// and the links between them, or session to session (Aria, 2026-11-07,
// item 348). Stations are not drawn in any of them.
type LineForm int

const (
	// FormDistricts runs only between districts, joining the nearest pair
	// of their buildings.
	FormDistricts LineForm = iota
	// FormNeighbourhoods outlines each neighbourhood and links one to
	// the next across districts.
	FormNeighbourhoods
	// FormSessions runs building to building, inside a district and
	// between them.
	FormSessions
)

func (f LineForm) String() string {
	switch f {
	case FormNeighbourhoods:
		return "neighbourhoods"
	case FormSessions:
		return "sessions"
	}
	return "districts"
}

// LineShape is what a line draws: segments in world space, each with the
// site it leads into, and outlines, each a neighbourhood's edges with
// the site it stands for.
type LineShape struct {
	Group string
	// Line is the line's place in the city's lines.
	Line         int
	Links        [][2]Point
	LinkSites    []LinkSite
	Outlines     [][][2]Point
	OutlineSites []int
}

// LinkSite is the site a link leads into, by its station's place in the
// line, and what that station said of it: unlit while the site waits on
// another, dashed when its sessions joined on a guess.
type LinkSite struct {
	Site          int
	Unlit, Dashed bool
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

// lineSiteOf is one of a line's sites as the forms draw it: its station's
// place in the line, and the group's buildings there.
type lineSiteOf struct {
	station   int
	buildings []*Building
}

// Shape is a line drawn in a form, in the line's site order. A site with
// no session of the group, one only the coordinator's subagents worked
// in, or a pull request opened outside it, is skipped: the line does not
// go to districts (item 348).
func (c *City) Shape(l Line, form LineForm) LineShape {
	shape := LineShape{Group: l.Group.ID}
	var sites []lineSiteOf
	seen := map[*District]bool{}
	for i, st := range l.Stations {
		if st.Teammate != nil || st.By != nil || seen[st.District] {
			continue
		}
		seen[st.District] = true
		if bs := st.buildings(l.Group); len(bs) > 0 {
			sites = append(sites, lineSiteOf{station: i, buildings: bs})
		}
	}
	into := func(s lineSiteOf) LinkSite {
		st := l.Stations[s.station]
		return LinkSite{Site: s.station, Unlit: st.Unlit, Dashed: st.Dashed}
	}
	link := func(a, b Point, s lineSiteOf) {
		shape.Links = append(shape.Links, [2]Point{a, b})
		shape.LinkSites = append(shape.LinkSites, into(s))
	}
	switch form {
	case FormSessions:
		var prev *Building
		if l.Office != nil {
			prev = l.Office.Building
		}
		for _, s := range sites {
			for _, b := range s.buildings {
				if b == prev {
					continue
				}
				if prev != nil {
					link(prev.Rect.Center(), b.Rect.Center(), s)
				}
				prev = b
			}
		}
	case FormDistricts:
		for i := 1; i < len(sites); i++ {
			p := nearestPair(sites[i-1].buildings, sites[i].buildings)
			link(p[0], p[1], sites[i])
		}
	case FormNeighbourhoods:
		var anchors []Point
		for _, s := range sites {
			anchor := s.buildings[0].Rect.Center()
			most := 0
			for _, n := range c.Neighbourhoods(c.DistrictOf(s.buildings[0])) {
				if n.Group != l.Group {
					continue
				}
				shape.Outlines = append(shape.Outlines, n.Edges())
				shape.OutlineSites = append(shape.OutlineSites, s.station)
				if len(n.Buildings) > most {
					anchor, most = n.Centre(), len(n.Buildings)
				}
			}
			anchors = append(anchors, anchor)
		}
		for i := 1; i < len(anchors); i++ {
			link(anchors[i-1], anchors[i], sites[i])
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

// SetLineForm sets how the groups' lines are drawn: at once, from the
// city already built (item 348).
func (s *Scene) SetLineForm(f LineForm) { s.lineForm = f }

// LineForm is how the groups' lines are drawn.
func (s *Scene) LineForm() LineForm { return s.lineForm }

// Shapes are the groups' lines in the scene's form, or none while the
// setting hides them. A group in one district has no link between
// districts, so it draws nothing in the districts form, and its chain or
// outline in the others.
func (s *Scene) Shapes() []LineShape {
	if s.linesHidden {
		return nil
	}
	var out []LineShape
	for i, l := range s.city.Lines {
		shape := s.city.Shape(l, s.lineForm)
		shape.Line = i
		if len(shape.Links)+len(shape.Outlines) > 0 {
			out = append(out, shape)
		}
	}
	return out
}

// linkAt is the link or outline edge of a group's line within tolerance
// of a world point, as the hit of the site it leads into or stands for,
// the nearest when several are (item 348).
func (s *Scene) linkAt(p Point, tolerance float64) (Hit, bool) {
	best, bestDist := Hit{}, tolerance
	found := false
	consider := func(line, site int, seg [2]Point) {
		if d := p.DistanceToSegment(seg[0], seg[1]); d <= bestDist {
			best, bestDist, found = Hit{Station: &StationHit{Line: &s.city.Lines[line], Index: site}}, d, true
		}
	}
	for _, shape := range s.Shapes() {
		for i, seg := range shape.Links {
			consider(shape.Line, shape.LinkSites[i].Site, seg)
		}
		for i, o := range shape.Outlines {
			for _, seg := range o {
				consider(shape.Line, shape.OutlineSites[i], seg)
			}
		}
	}
	return best, found
}

// ParseLineForm reads a form by the name group_line_form gives it.
func ParseLineForm(name string) (LineForm, bool) {
	for _, f := range []LineForm{FormDistricts, FormNeighbourhoods, FormSessions} {
		if f.String() == name {
			return f, true
		}
	}
	return FormDistricts, false
}
