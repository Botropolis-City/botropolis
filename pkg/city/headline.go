package city

import "botropolis.city/botropolis/pkg/state"

// HeadlineOf is the headline Summary would give once snapshot is laid
// out, read from the snapshot alone, so a city nobody is watching can
// keep its window title without laying anything out (item 197).
func (s *Scene) HeadlineOf(snapshot state.Snapshot) string {
	if s.veil != nil {
		snapshot = s.veil.Snapshot(snapshot)
	}
	var sum Summary
	for _, sess := range snapshot.Sessions {
		root := SessionRoot(sess)
		if root == "" || s.layout.IsHidden(root) || sess.Packed {
			continue
		}
		switch sess.State {
		case state.Working:
			sum.Working++
		case state.NeedsYou:
			if s.layout.SetAside(sess, s.now()) {
				sum.SetAside++
			} else {
				sum.NeedsYou++
			}
		case state.Waiting:
			sum.Waiting++
		case state.Unattended:
			sum.Unattended++
		case state.Resting:
			sum.Resting++
		}
	}
	return sum.Headline()
}
