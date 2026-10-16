package city

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// DiveRow is one station of the focused group, as the dive's list says
// it: the site, what its most urgent session last said, the newest pull
// request seen there, whose move it is and how long it has waited
// (item 90).
type DiveRow struct {
	Site     string
	Sentence string
	PR       string
	Move     state.Move
	Waited   time.Duration
	// Session is the site's lead: its most urgent session, the newest
	// on a tie.
	Session string
}

// Side is a side of the window.
type Side int

const (
	SideLeft Side = iota
	SideRight
)

func (s Side) String() string {
	if s == SideRight {
		return "right"
	}
	return "left"
}

// Dive is the focused group's stations in line order, or none while no
// group is focused.
func (s *Scene) Dive() []DiveRow {
	line, ok := s.focusedLine()
	if !ok {
		return nil
	}
	now := s.Clock()
	rows := make([]DiveRow, 0, len(line.Stations))
	for _, st := range line.Stations {
		rows = append(rows, diveRow(line.Group, st, now))
	}
	return rows
}

func (s *Scene) focusedLine() (Line, bool) {
	if s.focus == "" {
		return Line{}, false
	}
	for _, l := range s.city.Lines {
		if l.Group.ID == s.focus {
			return l, true
		}
	}
	return Line{}, false
}

func diveRow(g *state.Group, st Station, now time.Time) DiveRow {
	var lead *Building
	var last, prSeen time.Time
	pr := ""
	for _, b := range st.District.Buildings {
		if b.Group != g {
			continue
		}
		rank, leadRank := moveRank(b.Session), 0
		if lead != nil {
			leadRank = moveRank(lead.Session)
		}
		if lead == nil || rank > leadRank || (rank == leadRank && b.Session.StartedAt.After(lead.Session.StartedAt)) {
			lead = b
		}
		if b.Session.LastActivity.After(last) {
			last = b.Session.LastActivity
		}
		if n := len(b.Session.PRs); n > 0 && (pr == "" || b.Session.LastActivity.After(prSeen)) {
			pr, prSeen = prPhrase(b.Session.PRs[n-1]), b.Session.LastActivity
		}
	}
	row := DiveRow{Site: st.District.Name, PR: pr, Move: st.Move, Waited: now.Sub(last)}
	if lead != nil {
		row.Session, row.Sentence = lead.Session.ID, sentence(lead.Session, "")
	}
	return row
}

func prPhrase(p claude.PR) string {
	st := p.State
	if st == "" {
		st = claude.PROpen
	}
	return fmt.Sprintf("PR #%d %s", p.Number, st)
}

// moveRank orders a session's move by urgency: yours first, then the
// agents', someone else's and nobody's.
func moveRank(s state.Session) int {
	move, _ := state.GroupMove([]state.Session{s})
	switch move {
	case state.MoveYours:
		return 4
	case state.MoveAgents:
		return 3
	case state.MoveTheirs:
		return 2
	}
	return 1
}

// DiveStep is the row the dive is on, or -1 before any step.
func (s *Scene) DiveStep() int {
	if s.focus == "" {
		return -1
	}
	return s.dive - 1
}

// StepDive moves the dive by a row, selecting the site's lead session
// and centring the camera on its station; it stops at either end.
func (s *Scene) StepDive(by int) bool {
	line, ok := s.focusedLine()
	if !ok || len(line.Stations) == 0 {
		return false
	}
	i := max(0, min(s.dive-1+by, len(line.Stations)-1))
	if s.dive == 0 {
		i = 0
		if by < 0 {
			i = len(line.Stations) - 1
		}
	}
	s.dive = i + 1
	st := line.Stations[i]
	row := diveRow(line.Group, st, s.Clock())
	for _, b := range st.District.Buildings {
		if b.Session.ID == row.Session {
			s.selected = b
		}
	}
	s.CenterOn(st.At)
	return true
}

// LeaveDive leaves the focus and the selection the dive made with it.
func (s *Scene) LeaveDive() {
	s.ClearFocus()
	s.selected = nil
}

// SetDiveRoom is the room the dive's list takes, on a side of the
// window, reserved while a group is focused.
func (s *Scene) SetDiveRoom(side Side, px float64) {
	s.diveSide, s.diveRoom = side, px
}

// DiveSide is the side the dive's list docks on: away from the river,
// the plan's east edge, wherever the heading turns it.
func (s *Scene) DiveSide() Side {
	from, to, _, ok := s.city.RiverBand()
	if !ok {
		return SideLeft
	}
	river := (s.camera.WorldToScreen(from).X + s.camera.WorldToScreen(to).X) / 2
	if river < s.camera.WorldToScreen(s.city.Bounds().Center()).X {
		return SideRight
	}
	return SideLeft
}
