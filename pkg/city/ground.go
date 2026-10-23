package city

import (
	"slices"
	"sort"
)

// ground is what the standing layer draws from the city: every field
// pkg/render's composeStatic reads, walked by item 207, and nothing it
// does not. A field read there must be here, or a change to it is drawn
// only once something else moves the ground (item 206).
//
// It is derived from the city after every build rather than kept beside
// it, and holds roots and session IDs, never pointers, so two builds of
// one city hold equal grounds.
type ground struct {
	bounds      Rect
	riverCells  []RiverCell
	band        groundBand
	rails       []Point
	plaza       Rect
	streetCells []StreetCell
	// streets are each street's road, by index into roads, and its path,
	// laid end to end: streetEnds says where each path ends.
	streets    []int
	streetPath []Point
	streetEnds []int
	roads      []groundRoad
	towers     []groundTower
	// camps are each camp's lead and members' lots, laid end to end:
	// campEnds says where each camp's end.
	camps     []Rect
	campEnds  []int
	districts []groundLot
	pads      []Rect
	buildings []groundLot
	calls     []groundCall
}

type groundBand struct {
	from, to Point
	width    float64
	ok       bool
}

type groundRoad struct {
	a, b                                     Point
	from, to                                 string
	messages, files, backMessages, backFiles int
}

type groundTower struct {
	name string
	rect Rect
}

// groundLot is a district's or a building's lot, by its root or its
// session's ID.
type groundLot struct {
	id   string
	rect Rect
}

// groundCall is a session's calls to one server, which size and tint
// its beam.
type groundCall struct {
	id, server string
	calls      int
}

func groundOf(c *City) ground {
	g := ground{
		bounds:      c.Bounds(),
		riverCells:  slices.Clone(c.RiverCells),
		rails:       slices.Clone(c.Rails),
		plaza:       c.Plaza,
		streetCells: slices.Clone(c.StreetCells),
		pads:        c.Pads(c.StreetCells),
	}
	g.band.from, g.band.to, g.band.width, g.band.ok = c.RiverBand()
	road := map[*RoadLine]int{}
	for i := range c.Roads {
		r := &c.Roads[i]
		road[r] = i
		g.roads = append(g.roads, groundRoad{a: r.A, b: r.B, from: lotRoot(r.From), to: lotRoot(r.To),
			messages: r.Messages, files: r.Files, backMessages: r.BackMessages, backFiles: r.BackFiles})
	}
	for _, st := range c.Streets {
		i, ok := road[st.Road]
		if !ok {
			i = -1
		}
		g.streets = append(g.streets, i)
		g.streetPath = append(g.streetPath, st.Path...)
		g.streetEnds = append(g.streetEnds, len(g.streetPath))
	}
	for _, t := range c.Towers {
		g.towers = append(g.towers, groundTower{name: t.Server.Name, rect: t.Rect})
	}
	for _, camp := range c.Camps {
		if camp.Lead != nil {
			g.camps = append(g.camps, camp.Lead.Rect)
		}
		for _, m := range camp.Members {
			g.camps = append(g.camps, m.Rect)
		}
		g.campEnds = append(g.campEnds, len(g.camps))
	}
	for _, d := range c.Districts {
		g.districts = append(g.districts, groundLot{id: d.Root, rect: d.Rect})
		for _, b := range d.Buildings {
			g.buildings = append(g.buildings, groundLot{id: b.Session.ID, rect: b.Rect})
			start := len(g.calls)
			for server, n := range b.Session.MCPCalls {
				g.calls = append(g.calls, groundCall{id: b.Session.ID, server: server, calls: n})
			}
			calls := g.calls[start:]
			sort.Slice(calls, func(i, j int) bool { return calls[i].server < calls[j].server })
		}
	}
	return g
}

func lotRoot(d *District) string {
	if d == nil {
		return ""
	}
	return d.Root
}

func (a ground) equal(b ground) bool {
	return a.bounds == b.bounds && a.band == b.band && a.plaza == b.plaza &&
		slices.Equal(a.riverCells, b.riverCells) && slices.Equal(a.rails, b.rails) &&
		slices.Equal(a.streetCells, b.streetCells) && slices.Equal(a.streets, b.streets) &&
		slices.Equal(a.streetPath, b.streetPath) && slices.Equal(a.streetEnds, b.streetEnds) &&
		slices.Equal(a.roads, b.roads) && slices.Equal(a.towers, b.towers) &&
		slices.Equal(a.camps, b.camps) && slices.Equal(a.campEnds, b.campEnds) &&
		slices.Equal(a.districts, b.districts) && slices.Equal(a.pads, b.pads) &&
		slices.Equal(a.buildings, b.buildings) && slices.Equal(a.calls, b.calls)
}

// noteGround moves the ground's generation if what the standing layer
// draws has moved since it was last noted.
func (s *Scene) noteGround() {
	g := groundOf(s.city)
	if s.groundGeneration > 0 && g.equal(s.ground) {
		return
	}
	s.ground = g
	s.groundGeneration++
}

// GroundGeneration moves when what the standing layer draws moves: the
// plan, the streets and roads, the river, the rails, the lots, the
// towers and the beams' calls, the camps, and the detail (SetDetail moves it). A snapshot
// that moves only what stands on the ground leaves it where it was, so
// the renderer can keep the layer it painted (item 207).
func (s *Scene) GroundGeneration() int { return s.groundGeneration }
