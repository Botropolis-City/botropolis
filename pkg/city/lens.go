package city

import (
	"fmt"
	"slices"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Lens is a question the map can be asked (item 88). Each folds the
// views that answer it, opens on the first, and steps to the next when
// its key is pressed again. A lens decides what a card leads with and
// which action comes first.
type Lens int

const (
	LensAttention Lens = iota
	LensWork
	LensCost
	LensCapacity
	LensHealth
	LensConnections
)

// Lenses is every lens in the order its key numbers them.
var Lenses = []Lens{LensAttention, LensWork, LensCost, LensCapacity, LensHealth, LensConnections}

// LensPanelViewings is how many times a lens's panel opens in full
// before it shrinks to a chip.
const LensPanelViewings = 3

// capacityFirst is Capacity's first action, with a new session here
// beside it.
const capacityFirst = ActionFork

func (l Lens) Name() string {
	switch l {
	case LensWork:
		return "Work"
	case LensCost:
		return "Cost"
	case LensCapacity:
		return "Capacity"
	case LensHealth:
		return "Health"
	case LensConnections:
		return "Connections"
	}
	return "Attention"
}

// Views are the views the lens folds, the one it opens on first.
func (l Lens) Views() []View {
	switch l {
	case LensWork:
		return []View{ViewGroups}
	case LensCost:
		return []View{ViewSpend, ViewModels}
	case LensCapacity:
		return []View{ViewPressure, ViewFanout}
	case LensHealth:
		return []View{ViewHealth, ViewStaleness}
	case LensConnections:
		return []View{ViewServers, ViewTraffic}
	}
	return []View{ViewAttention}
}

// About is what the lens shows, in one sentence.
func (l Lens) About() string {
	switch l {
	case LensWork:
		return "Each piece of work and whose move it is: yours, the agents', or someone else's."
	case LensCost:
		return "What each session has spent, and on which models."
	case LensCapacity:
		return "How full each session's context is, and how many subagents it has running."
	case LensHealth:
		return "Errors, pull requests, and sessions that have gone quiet."
	case LensConnections:
		return "Which MCP servers each session calls, and which projects talk to each other."
	}
	return "Which sessions are waiting for you, and which are working."
}

// Do is what to do with the lens up, in one line.
func (l Lens) Do() string {
	switch l {
	case LensWork:
		return "Select a session to see its group; focus the line to see the rest of it."
	case LensCost:
		return "Select a session and press breakdown to see where its spend went."
	case LensCapacity:
		return "Select a full one and start a fresh session beside it."
	case LensHealth:
		return "Select a session with errors, or open its pull request."
	case LensConnections:
		return "Point at a tower to see who calls it; select a session to show its roads."
	}
	return "Press tab to go to the next session that needs you."
}

// first is the action the lens puts at the front of a card.
func (l Lens) first() ActionKind {
	switch l {
	case LensWork:
		return ActionFocusLine
	case LensCost:
		return ActionBreakdown
	case LensCapacity:
		return capacityFirst
	case LensHealth:
		return ActionOpenPR
	case LensConnections:
		return ActionShowRoad
	}
	return ActionAttach
}

// LensOf is the lens that holds a view.
func LensOf(v View) Lens {
	for _, l := range Lenses {
		if slices.Contains(l.Views(), v) {
			return l
		}
	}
	return LensAttention
}

// Lens is the lens the map is in.
func (s *Scene) Lens() Lens { return LensOf(s.view) }

// SelectLens is a lens's key: it opens the lens on its first view, or
// steps to its next view when the lens is already up.
func (s *Scene) SelectLens(l Lens) {
	views := l.Views()
	if s.Lens() == l {
		i := slices.Index(views, s.view)
		s.SetView(views[(i+1)%len(views)])
		return
	}
	s.SetView(views[0])
	s.lensExpanded = false
	s.seeLens(l)
}

func (s *Scene) seeLens(l Lens) {
	if s.layout == nil {
		return
	}
	if s.layout.Lenses == nil {
		s.layout.Lenses = map[string]int{}
	}
	s.layout.Lenses[l.Name()]++
}

// ShowLensPanel opens the lens's panel in full again.
func (s *Scene) ShowLensPanel() { s.lensExpanded = true }

// LensPanel is what the lens says about itself while it is up.
type LensPanel struct {
	Lens      Lens
	Name      string
	About     string
	Legend    Legend
	Aggregate string
	Do        string
	// Chip is the panel shrunk to its name, once it has been seen.
	Chip bool
}

func (s *Scene) LensPanel() LensPanel {
	l := s.Lens()
	seen := 0
	if s.layout != nil {
		seen = s.layout.Lenses[l.Name()]
	}
	return LensPanel{
		Lens:      l,
		Name:      l.Name(),
		About:     l.About(),
		Legend:    s.Legend(),
		Aggregate: s.aggregate(l),
		Do:        s.do(l),
		Chip:      seen > LensPanelViewings && !s.lensExpanded,
	}
}

// aggregate is the lens's one number for the whole city.
func (s *Scene) aggregate(l Lens) string {
	buildings := s.city.Buildings()
	count := func(keep func(state.Session) bool) int {
		n := 0
		for _, b := range buildings {
			if keep(b.Session) {
				n++
			}
		}
		return n
	}
	switch l {
	case LensWork:
		yours := 0
		for _, g := range s.snapshot.Groups {
			if g.Move == state.MoveYours {
				yours++
			}
		}
		groups := len(s.snapshot.Groups)
		if groups == 0 {
			return "no groups yet"
		}
		verb := "are"
		if groups == 1 {
			verb = "is"
		}
		return fmt.Sprintf("%d of %s %s your move", yours, plural(groups, "group"), verb)
	case LensCost:
		total, known, ranged := 0.0, false, false
		for _, b := range buildings {
			usd, ok, _ := b.spend()
			total += usd
			known = known || ok
			ranged = ranged || (ok && isClaude(b.Session) && b.Cost.Low < b.Cost.High)
		}
		if !known {
			return "spend not known yet"
		}
		upTo := ""
		if ranged {
			upTo = "up to "
		}
		return fmt.Sprintf("%s$%.2f across %s, since they started", upTo, total, plural(len(buildings), "session"))
	case LensCapacity:
		return plural(count(func(s state.Session) bool { return s.ContextPercent > 80 }), "session") + " over 80%"
	case LensHealth:
		return plural(count(func(s state.Session) bool { return s.APIErrors > 0 }), "session") + " with errors"
	case LensConnections:
		return plural(len(s.city.Roads), "road") + " between projects"
	}
	n := s.needingYou()
	if n == 1 {
		return "1 session needs you"
	}
	return fmt.Sprintf("%d sessions need you", n)
}

// JumpedTo is what tab, or a strip chip, says when it lands on a session.
func JumpedTo(st state.State, title string) string {
	words := string(st)
	if st == state.NeedsYou {
		words = "needs you"
	}
	return words + ": " + title
}

// NothingToJumpTo is what tab, or a strip chip, says when no session is
// in the state it looks for.
func NothingToJumpTo(st state.State) string {
	if st == state.NeedsYou {
		return "nobody needs you right now"
	}
	return fmt.Sprintf("nothing is %s", st)
}

// do is the panel's what-to-do line: Attention's follows its count, so
// it does not point tab at nobody.
func (s *Scene) do(l Lens) string {
	if l == LensAttention && s.needingYou() == 0 {
		return "Nobody is waiting on you; tab finds the next one when someone is."
	}
	return l.Do()
}

// prURL is the address of the session's pull request: the last one still
// open, else the last one it opened.
func prURL(sess state.Session) string {
	url := ""
	for _, pr := range sess.PRs {
		if pr.URL == "" {
			continue
		}
		if pr.State == "" || pr.State == claude.PROpen || url == "" {
			url = pr.URL
		}
	}
	return url
}

// roadsOf are the roads into the building's district.
func (s *Scene) roadsOf(b *Building) []*RoadLine {
	d := s.city.DistrictOf(b)
	var roads []*RoadLine
	for i := range s.city.Roads {
		if r := &s.city.Roads[i]; d != nil && (r.From == d || r.To == d) {
			roads = append(roads, r)
		}
	}
	return roads
}

// RoadsShown is the session whose roads show the road is lighting, or "".
func (s *Scene) RoadsShown() string {
	if s.selected == nil || s.roadFor != s.selected.Session.ID {
		return ""
	}
	return s.roadFor
}

// showRoad lights the roads into the selection's district and the
// districts at their other ends, once show the road was pressed.
func (s *Scene) showRoad(h *Highlight) bool {
	if s.selected == nil || s.roadFor != s.selected.Session.ID {
		return false
	}
	ends := map[*District]bool{}
	for _, r := range s.roadsOf(s.selected) {
		h.roads[r] = true
		ends[r.From], ends[r.To] = true, true
	}
	for _, b := range s.city.Buildings() {
		if ends[s.city.DistrictOf(b)] {
			h.buildings[b] = true
		}
	}
	h.Active = len(h.roads) > 0
	return h.Active
}
