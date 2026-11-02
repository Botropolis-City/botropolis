package state

import "time"

// QuietFor is whether a session has rested at least d, with its process
// closed and not yet packed up: what the daemon checks again before the
// quiet sweep packs it, whatever group it is in (item 333).
func (s Session) QuietFor(now time.Time, d time.Duration) bool {
	if s.State != Resting || s.Alive || s.Packed {
		return false
	}
	if s.LastActivity.IsZero() || s.LastActivity.After(now) {
		return false
	}
	return now.Sub(s.LastActivity) >= d
}

// Quiet is the sessions the quiet sweep packs up, by id, in their order:
// in no group, and quiet for at least d (item 333).
func Quiet(sessions []Session, now time.Time, d time.Duration) []string {
	var ids []string
	for i := range sessions {
		if sessions[i].Group == "" && sessions[i].QuietFor(now, d) {
			ids = append(ids, sessions[i].ID)
		}
	}
	return ids
}
