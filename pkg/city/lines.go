package city

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/format"
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
	// Office is the session coordinating the group, drawn as the line's
	// site office at its head (item 90), or nil.
	Office *Office
}

// Office is a line's site office: the coordinating session's building,
// and where on it the office stands.
type Office struct {
	Building *Building
	At       Point
}

// coordinator is the group's session whose subagents worked in, or
// whose messages reached, the most of the group's other districts, the
// earliest started on a tie; and the districts its subagents worked in
// where the group has no session, which become stations of its own.
func (c *City) coordinator(g *state.Group, bySite map[*District][]*Building) (*Building, []*District) {
	byRoot := map[string]*District{}
	for _, d := range c.Districts {
		byRoot[d.Root] = d
	}
	districtOf := map[string]*District{}
	for d, bs := range bySite {
		for _, b := range bs {
			districtOf[b.Session.ID] = d
		}
	}
	var best *Building
	var bestExtra []*District
	bestReach := 0
	for d, bs := range bySite {
		for _, b := range bs {
			reached := map[*District]bool{}
			var extra []*District
			for dir := range b.Session.Visits {
				v := byRoot[ProjectRoot(dir)]
				if v == nil || v == d {
					continue
				}
				if _, in := bySite[v]; in {
					reached[v] = true
				} else if !contains(extra, v) {
					extra = append(extra, v)
				}
			}
			for _, id := range b.Session.Messaged {
				if v := districtOf[id]; v != nil && v != d {
					reached[v] = true
				}
			}
			if len(reached) == 0 {
				continue
			}
			if len(reached) > bestReach || (len(reached) == bestReach && b.Session.StartedAt.Before(best.Session.StartedAt)) {
				best, bestReach, bestExtra = b, len(reached), extra
			}
		}
	}
	sort.Slice(bestExtra, func(i, j int) bool { return bestExtra[i].Root < bestExtra[j].Root })
	return best, bestExtra
}

func contains(ds []*District, d *District) bool {
	for _, x := range ds {
		if x == d {
			return true
		}
	}
	return false
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
	// Teammate is a pull request the group's sessions named but none of
	// them opened, drawn grey beside the site that named it (item 90).
	Teammate *Teammate
	// By is the coordinating session, at a district only its subagents
	// worked in, which speaks for the site there.
	By *Building
}

// buildings are the sessions that speak for a station's site.
func (st Station) buildings(g *state.Group) []*Building {
	if st.By != nil {
		return []*Building{st.By}
	}
	var out []*Building
	for _, b := range st.District.Buildings {
		if b.Group == g {
			out = append(out, b)
		}
	}
	return out
}

// Teammate is a pull request opened outside a group's sessions, as its
// station shows it.
type Teammate struct {
	PR          claude.PR
	MentionedIn string
	Seen        time.Time
	Age         string
}

// lineSite is one stop of a line: a district where the group has
// sessions, or a pull request opened outside them.
type lineSite struct {
	district *District
	teammate *group.TeammatePR
}

func (l lineSite) name() string {
	if l.teammate != nil {
		return fmt.Sprintf("%s#%d", l.teammate.PR.Repository, l.teammate.PR.Number)
	}
	return l.district.Name
}

// station is where a line stops at a site, and whose move it is there.
func (c *City) station(site lineSite, bySite map[*District][]*Building, through map[*District]int) Station {
	d := site.district
	if t := site.teammate; t != nil {
		pr := claude.PR{Number: t.PR.Number, Repository: t.PR.Repository, URL: t.PR.URL, State: claude.PROpen}
		switch {
		case t.PR.Merged:
			pr.State = claude.PRMerged
		case t.Closed:
			pr.State = claude.PRClosed
		}
		at := d.Rect.Center()
		at = Point{X: at.X + float64(through[d])*stationSpacing*d.Rect.Width(), Y: at.Y + stationSpacing*d.Rect.Height()}
		through[d]++
		move := state.MoveTheirs
		if pr.State != claude.PROpen {
			move = state.MoveNobody
		}
		return Station{District: d, At: at, Move: move, Teammate: &Teammate{PR: pr, MentionedIn: d.Name, Seen: t.Seen, Age: format.Age(c.Time.Sub(t.Seen))}}
	}
	sessions := make([]state.Session, 0, len(bySite[d]))
	dashed := true
	for _, b := range bySite[d] {
		sessions = append(sessions, b.Session)
		dashed = dashed && b.Session.GroupLikely
	}
	move, _ := state.GroupMove(sessions)
	var centre Point
	if len(bySite[d]) == 0 {
		// A district only the coordinator's subagents worked in.
		centre, dashed = d.Rect.Center(), false
	}
	for _, b := range bySite[d] {
		c := b.Rect.Center()
		centre.X += c.X / float64(len(bySite[d]))
		centre.Y += c.Y / float64(len(bySite[d]))
	}
	at := Point{X: centre.X + float64(through[d])*stationSpacing*d.Rect.Width(), Y: centre.Y}
	through[d]++
	return Station{District: d, At: at, Move: move, Dashed: dashed}
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
		office, visited := c.coordinator(g, bySite)
		sites := make([]*District, 0, len(bySite)+len(visited))
		for d := range bySite {
			sites = append(sites, d)
		}
		for _, d := range visited {
			sites = append(sites, d)
			started[d] = office.Session.StartedAt
		}
		sort.Slice(sites, func(i, j int) bool {
			if !started[sites[i]].Equal(started[sites[j]]) {
				return started[sites[i]].Before(started[sites[j]])
			}
			return sites[i].Root < sites[j].Root
		})
		ordered, waits := orderSites(sites, bySite, started)
		line := Line{Group: g}
		if office != nil {
			r := office.Rect
			line.Office = &Office{Building: office, At: Point{X: r.Min.X + r.Width()*0.25, Y: r.Min.Y + r.Height()*0.25}}
		}
		for i, site := range ordered {
			st := c.station(site, bySite, through)
			if len(bySite[site.district]) == 0 && site.teammate == nil {
				st.Move, _ = state.GroupMove([]state.Session{office.Session})
				st.By = office
			}
			for _, w := range waits {
				if w.Site != i {
					continue
				}
				if w.Done {
					st.Waits = append(st.Waits, "unblocked: "+w.Unblocked)
				} else {
					st.Unlit = true
					st.Waits = append(st.Waits, "waits on "+ordered[w.On].name()+" until "+w.Until)
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
		names = append(names, st.Name())
	}
	st := l.Stations[i]
	if t := st.Teammate; t != nil {
		return Card{Title: l.Group.Name, Lines: append([]string{
			fmt.Sprintf("pr #%d %s, opened outside these sessions, %s, seen %s ago in %s", t.PR.Number, t.PR.Repository, t.PR.State, t.Age, t.MentionedIn),
			"line     " + strings.Join(names, " → "),
		}, st.Waits...)}
	}
	return Card{Title: l.Group.Name, Lines: append([]string{
		"station  " + st.District.Name + ", " + string(st.Move),
		"line     " + strings.Join(names, " → "),
	}, st.Waits...)}
}

// Name is what a station is called on its line.
func (st Station) Name() string {
	if t := st.Teammate; t != nil {
		return fmt.Sprintf("%s#%d", t.PR.Repository, t.PR.Number)
	}
	return st.District.Name
}

// orderSites puts a line's sites, given in the order they were started,
// in the order they can be read to depend on (item 90), and says which
// waits on which, indexed by the order returned.
func orderSites(sites []*District, bySite map[*District][]*Building, started map[*District]time.Time) ([]lineSite, []group.Wait) {
	in := make([]group.OrderSite, 0, len(sites))
	var own []group.SitePR
	var mentions []group.Mention
	for i, d := range sites {
		site := group.OrderSite{Name: d.Name, Started: started[d]}
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
			own = append(own, site.PRs...)
			for _, l := range s.OrderLines {
				mentions = append(mentions, group.Mention{Site: i, Text: l.Text, Command: l.Command, At: l.At})
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
	all := make([]lineSite, 0, len(sites))
	for _, d := range sites {
		all = append(all, lineSite{district: d})
	}
	teammates := group.TeammatePRs(own, mentions)
	for i := range teammates {
		t := &teammates[i]
		all = append(all, lineSite{district: sites[t.Site], teammate: t})
		in = append(in, group.OrderSite{Name: all[len(all)-1].name(), Started: t.Seen, PRs: []group.SitePR{t.PR}, Done: t.PR.Merged || t.Closed})
	}
	order, waits := group.Order(in)
	at := make([]int, len(order))
	out := make([]lineSite, len(order))
	for to, from := range order {
		out[to], at[from] = all[from], to
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
