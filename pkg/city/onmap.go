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
	if n == len(snapshot.Sessions) {
		return snapshot, nil
	}
	kept := make([]state.Session, 0, n)
	var off []state.Session
	for _, sess := range snapshot.Sessions {
		if sh.Draws(sess) {
			kept = append(kept, sess)
		} else {
			off = append(off, sess)
		}
	}
	snapshot.Sessions = kept
	return snapshot, off
}

// Draws is whether the map draws a session.
func (sh Shown) Draws(sess state.Session) bool {
	return (sh.Gone || !sess.ConversationGone) && (sh.Scripted || !sess.Scripted())
}
