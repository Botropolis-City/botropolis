package render

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// openCeiling is the longest a capture waits for its history to be read.
// A capture reads at full speed (item 240); the ceiling is there so a
// history that never reads cannot hold a film or a frame test for ever
// (item 202).
const openCeiling = 30 * time.Second

// ceilingMode is what openCeiling bounds: the whole wait, or the time
// since the read last moved (item 249).
type ceilingMode int

const (
	totalCeiling ceilingMode = iota
	stallCeiling
)

func (m ceilingMode) String() string {
	if m == stallCeiling {
		return "stall"
	}
	return "total"
}

// openCeilingMode is the mode BOTROPOLIS_FRAME_OPEN_CEILING asks for,
// which only a devtools build reads: the acceptance harness asks for
// stall, so a machine's load cannot fail a read that is slow but
// advancing.
func openCeilingMode(value string) ceilingMode {
	if value == "stall" {
		return stallCeiling
	}
	return totalCeiling
}

// readProgress is how far a capture's read had got when it last moved:
// the fewest sessions seen with a partial heft, and the most of a long
// read's scan done. Either is in every snapshot, from a daemon or not.
type readProgress struct {
	seen    bool
	partial int
	done    int64
	moved   time.Time
	// latest is the newest snapshot's partial hefts, and since how many
	// snapshots have come without the read moving, for a stall's
	// failure to say what it saw.
	latest, since int
}

func (p readProgress) String() string {
	return fmt.Sprintf("%d sessions' hefts still partial, %d bytes of the scan done, %d snapshots since it last moved", p.latest, p.done, p.since)
}

// note takes a snapshot's progress, and moves the clock when the read
// has got further than ever before.
func (p *readProgress) note(snapshot *state.Snapshot, now time.Time) {
	if snapshot == nil {
		return
	}
	partial := 0
	for _, s := range snapshot.Sessions {
		if s.HeftPartial {
			partial++
		}
	}
	var done int64
	if snapshot.Scan != nil {
		done = snapshot.Scan.Done
	}
	p.latest = partial
	if p.seen && partial >= p.partial && done <= p.done {
		p.since++
		return
	}
	p.since = 0
	if !p.seen || partial < p.partial {
		p.partial = partial
	}
	p.done = max(p.done, done)
	p.seen = true
	p.moved = now
}

// holdUnopened takes the feed's newest snapshot for a capture whose city
// has not opened, without ticking, and reports whether it is still to
// wait. A recording's clock waits for each tick to be painted, and an
// unopened city paints nothing of the city, so a wait inside the tick
// would never end. At the ceiling it fails the capture instead.
func (g *Game) holdUnopened(now time.Time) (bool, error) {
	if g.scene == nil || !g.capturing() || g.ticks == 0 || !g.scene.WaitingForRead() {
		return false, nil
	}
	g.mu.Lock()
	pending := g.pending
	g.pending = nil
	g.mu.Unlock()
	g.readProgress.note(pending, now)
	g.takeSnapshot(pending)
	if err := g.unreadAtCeiling(now); err != nil {
		return false, err
	}
	return !g.scene.Opened(), nil
}

// unreadAtCeiling is the error that ends a capture whose city has waited
// openCeiling on a history still unread: a frame taken from a partial
// read would be wrong without saying so (item 240). Under the stall
// ceiling it waits openCeiling from the read's last move instead
// (item 249).
func (g *Game) unreadAtCeiling(now time.Time) error {
	if g.scene == nil || !g.capturing() || !g.scene.WaitingForRead() {
		return nil
	}
	if g.ceiling == stallCeiling {
		moved := g.started
		if g.readProgress.seen {
			moved = g.readProgress.moved
		}
		if now.Sub(moved) < openCeiling {
			return nil
		}
		return fmt.Errorf("the history was not read: the read made no progress for %s (%s), so no frame is taken from a partial read", openCeiling, g.readProgress)
	}
	if now.Sub(g.started) < openCeiling {
		return nil
	}
	return fmt.Errorf("the history was not read within %s, so no frame is taken from a partial read", openCeiling)
}
