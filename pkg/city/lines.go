package city

import (
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/group"
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
	// Unlit is a site waiting on another until an event not yet seen,
	// and Waits what it waits on or what unblocked it (item 90).
	Unlit bool
	Waits []string
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
		sites, waits := orderSites(sites, bySite)
		line := Line{Group: g}
		for i, d := range sites {
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
			st := Station{District: d, At: at, Move: move, Dashed: dashed}
			for _, w := range waits {
				if w.Site != i {
					continue
				}
				if w.Done {
					st.Waits = append(st.Waits, "unblocked: "+w.Unblocked)
				} else {
					st.Unlit = true
					st.Waits = append(st.Waits, "waits on "+sites[w.On].Name+" until "+w.Until)
				}
			}
			line.Stations = append(line.Stations, st)
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
	return Card{Title: l.Group.Name, Lines: append([]string{
		"station  " + st.District.Name + ", " + string(st.Move),
		"line     " + strings.Join(names, " → "),
	}, st.Waits...)}
}

// orderSites puts a line's sites, given in the order they were started,
// in the order they can be read to depend on (item 90), and says which
// waits on which, indexed by the order returned.
func orderSites(sites []*District, bySite map[*District][]*Building) ([]*District, []group.Wait) {
	in := make([]group.OrderSite, 0, len(sites))
	for _, d := range sites {
		site := group.OrderSite{Name: d.Name}
		var sessions []state.Session
		var recapAt time.Time
		for _, b := range bySite[d] {
			s := b.Session
			sessions = append(sessions, s)
			if site.Started.IsZero() || s.StartedAt.Before(site.Started) {
				site.Started = s.StartedAt
			}
			for _, pr := range s.PRs {
				site.PRs = append(site.PRs, group.SitePR{Repository: pr.Repository, Number: pr.Number, URL: pr.URL, Merged: pr.Merged()})
			}
			for _, l := range s.OrderLines {
				if l.Command {
					site.Commands = append(site.Commands, l.Text)
				} else {
					site.Words = append(site.Words, l.Text)
				}
			}
			if s.LastPrompt != "" {
				site.Words = append(site.Words, s.LastPrompt)
			}
			if s.Recap.Text != "" && !s.Recap.At.Before(recapAt) {
				site.Recap, recapAt = s.Recap.Text, s.Recap.At
			}
		}
		move, _ := state.GroupMove(sessions)
		site.Done = move == state.MoveNobody
		in = append(in, site)
	}
	order, waits := group.Order(in)
	at := make([]int, len(order))
	out := make([]*District, len(order))
	for to, from := range order {
		out[to], at[from] = sites[from], to
	}
	for i := range waits {
		waits[i].Site, waits[i].On = at[waits[i].Site], at[waits[i].On]
	}
	return out, waits
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
