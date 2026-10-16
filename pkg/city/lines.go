package city

import (
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// A line is a group drawn across the districts it runs through, the way
// Beck drew the Underground: a station at each district where it has
// sessions, in the order its sites were started, the order mattering
// and the distance not. The line is neutral ink and its stations carry
// the tones of whose move each site is (item 87).
type Line struct {
	Group    *state.Group
	Stations []Station
}

type Station struct {
	District *District
	// At is where the station stands, in world space: among the group's
	// own buildings in the district, not the middle of its lot.
	At   Point
	Move state.Move
	// Dashed is a site whose sessions all joined on a guess.
	Dashed bool
}

// stationSpacing is how far apart the stations of different lines stand
// in a district they share, as a share of the district's width.
const stationSpacing = 0.12

func (c *City) placeLines() {
	c.Lines = nil
	through := map[*District]int{}
	for _, g := range c.groups {
		bySite := map[*District][]*Building{}
		started := map[*District]time.Time{}
		for _, d := range c.Districts {
			for _, b := range d.Buildings {
				if b.Group != g {
					continue
				}
				bySite[d] = append(bySite[d], b)
				if t, ok := started[d]; !ok || b.Session.StartedAt.Before(t) {
					started[d] = b.Session.StartedAt
				}
			}
		}
		if len(bySite) == 0 {
			continue
		}
		sites := make([]*District, 0, len(bySite))
		for d := range bySite {
			sites = append(sites, d)
		}
		sort.Slice(sites, func(i, j int) bool {
			if !started[sites[i]].Equal(started[sites[j]]) {
				return started[sites[i]].Before(started[sites[j]])
			}
			return sites[i].Root < sites[j].Root
		})
		line := Line{Group: g}
		for _, d := range sites {
			sessions := make([]state.Session, 0, len(bySite[d]))
			dashed := true
			for _, b := range bySite[d] {
				sessions = append(sessions, b.Session)
				dashed = dashed && b.Session.GroupLikely
			}
			move, _ := state.GroupMove(sessions)
			var centre Point
			for _, b := range bySite[d] {
				c := b.Rect.Center()
				centre.X += c.X / float64(len(bySite[d]))
				centre.Y += c.Y / float64(len(bySite[d]))
			}
			at := Point{X: centre.X + float64(through[d])*stationSpacing*d.Rect.Width(), Y: centre.Y}
			through[d]++
			line.Stations = append(line.Stations, Station{District: d, At: at, Move: move, Dashed: dashed})
		}
		c.Lines = append(c.Lines, line)
	}
}

// StationCard is what a station says when the pointer is on it: its
// group, the site and whose move it is there, and the line's route.
func StationCard(l Line, i int) Card {
	names := make([]string, 0, len(l.Stations))
	for _, st := range l.Stations {
		names = append(names, st.District.Name)
	}
	st := l.Stations[i]
	return Card{Title: l.Group.Name, Lines: []string{
		"station  " + st.District.Name + ", " + string(st.Move),
		"line     " + strings.Join(names, " → "),
	}}
}

// Lines are the groups' lines, or none while the setting hides them.
func (s *Scene) Lines() []Line {
	if s.linesHidden {
		return nil
	}
	return s.city.Lines
}

// SetGroupLines shows or hides the groups' lines; they are on by
// default.
func (s *Scene) SetGroupLines(on bool) {
	s.linesHidden = !on
}

func (s *Scene) GroupLines() bool {
	return !s.linesHidden
}
