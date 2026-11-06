package city

import "botropolis.city/botropolis/pkg/state"

// Shown is which sessions the map draws that it leaves off by default.
// Leaving one off takes only its building: the snapshot, the ledger and
// the statistics keep it, so turning it back on needs no reload.
type Shown struct {
	// Gone draws sessions whose transcript Claude Code deleted (item 274).
	Gone bool
	// Scripted draws sessions a program started, claude -p and the Agent
	// SDK (item 273).
	Scripted bool
}

// OffMap names the usage of sessions the map leaves off, in a breakdown
// read without the statistics store (item 273).
const OffMap = "off the map"

// SetShown changes which sessions the map draws, at once.
func (s *Scene) SetShown(shown Shown) {
	if s.shown == shown {
		return
	}
	s.shown = shown
	if s.offered {
		s.restate()
	}
}

// Shown is which sessions the map draws that it leaves off by default.
func (s *Scene) Shown() Shown { return s.shown }

// onMap is the snapshot with the sessions the map leaves off taken out,
// and those sessions, whose usage still counts.
func (sh Shown) onMap(snapshot state.Snapshot) (state.Snapshot, []state.Session) {
	n := 0
	for _, sess := range snapshot.Sessions {
		if sh.Draws(sess) {
			n++
		}
	}
	var off []state.Session
	if n < len(snapshot.Sessions) {
		kept := make([]state.Session, 0, n)
		for _, sess := range snapshot.Sessions {
			if sh.Draws(sess) {
				kept = append(kept, sess)
			} else {
				off = append(off, sess)
			}
		}
		snapshot.Sessions = kept
	}
	snapshot.Groups = groupsOnMap(snapshot.Groups, snapshot.Sessions)
	return snapshot, off
}

// groupsOnMap is each group with only the members the map draws, and
// without the groups none of whose sessions it draws (item 333). A group
// with one session on the map is shown, not hidden (item 344). What it
// changes, it copies, so the snapshot it was given stays as it was.
func groupsOnMap(groups []state.Group, sessions []state.Session) []state.Group {
	if len(groups) == 0 {
		return groups
	}
	on := make(map[string]bool, len(sessions))
	carrying := map[string]int{}
	for _, s := range sessions {
		on[s.ID] = true
		if s.Group != "" {
			carrying[s.Group]++
		}
	}
	kept := make([]state.Group, 0, len(groups))
	for _, g := range groups {
		if carrying[g.ID] == 0 {
			continue
		}
		if !every(g.Members, on) {
			g = membersOn(g, on)
		}
		kept = append(kept, g)
	}
	return kept
}

func every(ids []string, on map[string]bool) bool {
	for _, id := range ids {
		if !on[id] {
			return false
		}
	}
	return true
}

func membersOn(g state.Group, on map[string]bool) state.Group {
	members, dirs, titles := g.Members, g.MemberDirs, g.MemberTitles
	g.Members, g.MemberDirs, g.MemberTitles = nil, nil, nil
	for i, id := range members {
		if !on[id] {
			continue
		}
		g.Members = append(g.Members, id)
		if i < len(dirs) {
			g.MemberDirs = append(g.MemberDirs, dirs[i])
		}
		if i < len(titles) {
			g.MemberTitles = append(g.MemberTitles, titles[i])
		}
	}
	return g
}

// Draws is whether the map draws a session.
func (sh Shown) Draws(sess state.Session) bool {
	return (sh.Gone || !sess.ConversationGone) && (sh.Scripted || !sess.Scripted())
}
