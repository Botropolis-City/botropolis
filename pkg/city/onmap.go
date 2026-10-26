package city

import "botropolis.city/botropolis/pkg/state"

// Shown is which sessions the map draws that it leaves off by default.
// Leaving one off takes only its building: the snapshot, the ledger and
// the statistics keep it, so turning it back on needs no reload.
type Shown struct {
	// Gone draws sessions whose transcript Claude Code deleted (item 274).
	Gone bool
}

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

// onMap is the snapshot with the sessions the map leaves off taken out.
func (sh Shown) onMap(snapshot state.Snapshot) state.Snapshot {
	n := 0
	for _, sess := range snapshot.Sessions {
		if sh.draws(sess) {
			n++
		}
	}
	if n == len(snapshot.Sessions) {
		return snapshot
	}
	kept := make([]state.Session, 0, n)
	for _, sess := range snapshot.Sessions {
		if sh.draws(sess) {
			kept = append(kept, sess)
		}
	}
	snapshot.Sessions = kept
	return snapshot
}

func (sh Shown) draws(sess state.Session) bool {
	return sh.Gone || !sess.ConversationGone
}
